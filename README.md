# Nakama <> Edgegap Integration

Deploy dedicated game engine servers for popular engines (e.g., Unity, Unreal, etc.) or custom engine servers, fully integrated with Nakama's open-source player data and game services for a convenient turnkey solution.

## Edgegap Setup

To prepare your dedicated game server build for deployment on Edgegap, see:
- [Getting Started with Servers (Unity)](https://docs.edgegap.com/learn/unity-games/getting-started-with-servers)
- [Getting Started with Servers (Unreal Engine)](https://docs.edgegap.com/learn/unreal-engine-games/getting-started-with-servers)

## Installation 

To add the Edgegap Fleet Manager to your Nakama go plugin project, use the following command:
```shell
go get github.com/edgegap/nakama-edgegap
```

This will add it to your project's `go.mod` dependencies.

## Nakama Setup

You must set up the following Environment Variable inside your Nakama's cluster:
```shell
EDGEGAP_API_URL=https://api.edgegap.com
EDGEGAP_API_TOKEN=<The Edgegap's API Token (keep the 'token' in the API Token)>
EDGEGAP_APPLICATION=<The Edgegap's Application Name to use to deploy>
EDGEGAP_VERSION=<The Edgegap's Version Name to use to deploy>
EDGEGAP_PORT_NAME=<The Edgegap's Application Port Name to send to game client>
NAKAMA_ACCESS_URL=<Nakama API Url, for Heroic Cloud, it will be provided when you create your instance>
```

You can copy the `local.yml.example` to `local.yml` and fill it out to start with your local cluster

Optional Values with default
```shell
EDGEGAP_POLLING_INTERVAL=<Interval where Nakama will sync with Edgegap API in case of mistmach (default:15m ) >
NAKAMA_CLEANUP_INTERVAL==<Interval where Nakama will check reservations expiration (default:1m )
NAKAMA_RESERVATION_MAX_DURATION==<Max Duration of a reservations before it expires (default:30s )
```

Using the Nakama's Storage Index and basic struct Instance Info,
we store extra information in the metadata for Edgegap using 2 list.
1 list to holds seats reservations
1 list to holds active connections

Using Max Players field we can now create the field `AvailableSeats` that will be in sync with that (
MaxPlayers-Reservations-Connections=AvailableSeats)

## Usage

from your `main.go` where the `InitModule` global function is, you need to register the Fleet Manager

```go
// Register the Fleet Manager
efm, err := fleetmanager.NewEdgegapFleetManager(ctx, logger, db, nk, initializer)
if err != nil {
    return err
}

if err = initializer.RegisterFleetManager(efm); err != nil {
    logger.WithField("error", err).Error("failed to register Edgegap fleet manager")
    return err
}
```

you can use the `main.go` from this project and also copy the `local.yml.example` to start a local Nakama using docker compose.

copy `docker-compose.yml` and `Dockerfile` to the root of your project and run the following command to start a local cluster:
```shell
docker compose up --build -d
```

Run the following command to stop it
```shell
docker compose down
```

## Server Placement

Game clients only interact with Edgegap APIs through Nakama RPCs, defaulting to [Nakama authentication method of your choice](https://heroiclabs.com/docs/nakama/concepts/authentication/). [Edgegap's Server Placement utilizing Server Score strategy](https://docs.edgegap.com/learn/advanced-features/deployments#1-server-score-strategy-best-practice) uses public IP addresses of participating players to choose the optimal server location. To store the player IP address and pass it to Edgegap when looking for server, store player's public IP in their Profile's Metadata as `PlayerIP`.

In your `main.go`, during the Init you can add the Registration of the Authentication of the type you implemented

```go
    // Register Authentication Methods
    if err := initializer.RegisterAfterAuthenticateCustom(fleetmanager.OnAuthenticateUpdateCustom); err != nil {
        return err
    }
```

This will automatically store in Profile's Metadata the `PlayerIP`

## Instance Server -> Nakama

When using this integration, every Instance made through Edgegap's platform will have many Environment Variables
injected into the Game Server.

This the responsibility of the Game Server to trigger events when specific actions are triggered to communicate
back to the Nakama's cluster to allow the Instance to be in sync and allow Players to connect to those instance.

### Injected Environment Variables

The following Environment Variables will be available in the game server:

- `NAKAMA_CONNECTION_EVENT_URL` (url to send connection events of the players)
- `NAKAMA_INSTANCE_EVENT_URL` (url to send instance event actions)
- `NAKAMA_INSTANCE_METADATA` (contains create metadata JSON)

### Connection Events

using `NAKAMA_CONNECTION_EVENT_URL` you must send all connection event (new connections, disconnections)
to the Nakama instance with the following body:

```json
{
  "instance_id": "<instance_id>",
  "connections": [
    "<user_id>"
  ]
}
```

`connections` is the list of active user IDs connected to the instance server, on event change like
disconnections/reconnections
simply send the updated list on each event.

### Instance Events

using `NAKAMA_INSTANCE_EVENT_URL` you must send instance action (when the Game Server is ready to receive connections,
in error or when it wants to stop)
to the Nakama instance with the following body:

```json
{
  "instance_id": "<instance_id>",
  "action": "[READY|ERROR|STOP]",
  "message": "",
  "metadata": {}
}
```

The `action` must be one of the following:

- READY (will mark the instance as ready and trigger callback event to notify players)
- ERROR (will mark the instance in error and trigger callback event to notify players)
- STOP (will call Edgegap's API to stop the running deployment)

The field `message` is to provide extra data (most likely for Error Action)

The field `metadata` will be merged to the metadata of the instance info (instance)

## Game Client -> Nakama (optional rpc)

This is an optional and included Client RPC route to do basic operation on Instance like listing, creating and joining.
Feel free to create your own or use the matchmaker instead of this.

### Create Instance

RPC - instance_create

```json
{
  "max_players": 2,
  "user_ids": [],
  "metadata": {}
}
```

`max_players` to -1 for unlimited

if `user_ids` is empty, the user's ID calling this will be used

### Get Instance

RPC - instance_get

```json
{
  "instance_id": "<instance_id>"
}
```

### List Instance

RPC - instance_list

```json
{
  "query": "",
  "limit": 100,
  "cursor": ""
}
```

`query` can be used to search instance with available seats

Example to list all instances READY with at least 1 seat available

```json
{
  "query": "+value.metadata.edgegap.available_seats:>=1 +value.status:READY",
  "limit": 100,
  "cursor": ""
}

```

### Join Instance

RPC - instance_join

```json
{
  "instance_id": "<instance_id>",
  "user_ids": []
}
```

if `user_ids` is empty, the user's ID calling this will be used


## Matchmaker

You can create your own integration using the Nakama's Matchmaker, this is an example on how you could do it:

```go
// OnMatchmakerMatched When a match is created via matchmaker, collect the Users and create a instance
func OnMatchmakerMatched(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, entries []runtime.MatchmakerEntry) (string, error) {
	maxPlayers := len(entries)
	userIds := make([]string, 0, len(entries))
	for _, entry := range entries {
		userIds = append(userIds, entry.GetPresence().GetUserId())
	}
	
	var callback runtime.FmCreateCallbackFn = func(status runtime.FmCreateStatus, instanceInfo *runtime.InstanceInfo, sessionInfo []*runtime.SessionInfo, metadata map[string]any, createErr error) {
		switch status {
		case runtime.CreateSuccess:
			logger.Info("Edgegap instance created: %s", instanceInfo.Id)

			content := map[string]interface{}{
				"IpAddress":  instanceInfo.ConnectionInfo.IpAddress,
				"DnsName":    instanceInfo.ConnectionInfo.DnsName,
				"Port":       instanceInfo.ConnectionInfo.Port,
				"InstanceId": instanceInfo.Id,
			}
			// Send connection details notifications to players
			for _, userId := range userIds {
				subject := "connection-info"

				code := notificationConnectionInfo
				err := nk.NotificationSend(ctx, userId, subject, content, code, "", false)
				if err != nil {
					logger.WithField("error", err.Error()).Error("Failed to send notification")
				}
			}
			return
		case runtime.CreateTimeout:
			logger.WithField("error", createErr.Error()).Error("Failed to create Edgegap instance, timed out")

			// Send notification to client that instance session creation timed out
			for _, userId := range userIds {
				subject := "create-timeout"
				content := map[string]interface{}{}
				code := notificationCreateTimeout
				err := nk.NotificationSend(ctx, userId, subject, content, code, "", false)
				if err != nil {
					logger.WithField("error", err.Error()).Error("Failed to send notification")
				}
			}
		default:
			logger.WithField("error", createErr.Error()).Error("Failed to create Edgegap instance")

			// Send notification to client that instance session couldn't be created
			for _, userId := range userIds {
				subject := "create-failed"
				content := map[string]interface{}{}
				code := notificationCreateFailed
				err := nk.NotificationSend(ctx, userId, subject, content, code, "", false)
				if err != nil {
					logger.WithField("error", err.Error()).Error("Failed to send notification")
				}
			}
			return
		}
	}
	
	efm := nk.GetFleetManager()
	err := efm.Create(ctx, maxPlayers, userIds, nil, nil, callback)

	reply := instanceCreateReply{
		Message: "Instance Created",
		Ok:      true,
	}

	replyString, err := json.Marshal(reply)
	if err != nil {
		logger.WithField("error", err.Error()).Error("failed to marshal instance create reply")
		return "", ErrInternalError
	}

	return string(replyString), err
}
```

and register it like this in the `InitModule` function in your `main.go`

```go
err = initializer.RegisterMatchmakerMatched(OnMatchmakerMatched)
if err != nil {
    logger.WithField("error", err).Error("failed to register Matchmaker matched with fleet manager")
    return err
}
```