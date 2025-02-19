# Nakama <> Edgegap Integration

## Concept

Using the Nakama's Storage Index and basic struct Instance Info,
we store extra information in the metadata for Edgegap using 2 list.
1 list to holds seats reservations
1 list to holds active connections

Using Max Players field we can now create the field `AvailableSeats` that will be in sync with that (
MaxPlayers-Reservations-Connections=AvailableSeats)

## Setup

TODO 

- Containerize Game
- Create Edgegap's Account
- Create Application and Version
- Create API Token

## Nakama Setup

You must set up the following Environment Variable inside your Nakama's cluster:

- EDGEGAP_API_URL=https://api.edgegap.com
- EDGEGAP_API_TOKEN=<The Edgegap's API Token (keep the 'token' in the API Token)
- EDGEGAP_APPLICATION=<The Edgegap's Application Name to use to deploy>
- EDGEGAP_VERSION=<The Edgegap's Version Name to use to deploy>
- EDGEGAP_PORT_NAME=<The Edgegap's Application Port Name to send to game client>
- NAKAMA_ACCESS_URL=<Nakama API Url, for Heroic Cloud, it will be provided when you create your instance>

You can copy the `local.yml.example` to `local.yml` and fill it out to start with your local cluster

## Authentication

Edgegap's API handle IPs to determine the best possible locations for the players, to allow the Game Client's IP to
be retrieved, some methods are offered to simplify the integration.

In your `main.go`, during the Init you can add the Registration of the Authentication of the type you implemented

```go
    // Register Authentication Methods
if err := initializer.RegisterAfterAuthenticateCustom(fleetmanager.OnAuthenticateUpdateCustom); err != nil {
return err
}
```

This will automatically store in Profile's Metadata the `PlayerIP`

## Game Server -> Nakama

TODO

### Injected Environment Variables

The following Environment Variables will be available in the game server:

- `NAKAMA_CONNECTION_EVENT_URL` (url to send connection events of the players)
- `NAKAMA_GAME_EVENT_URL` (url to send game event actions)
- `NAKAMA_GAME_METADATA` (contains create metadata JSON)

### Connection Events

using `NAKAMA_CONNECTION_EVENT_URL` you must send all connection event
to the Nakama instance with the following body:

```json
{
  "game_id": "<game_id>",
  "connections": [
    "<user_id>"
  ]
}
```

`connections` is the list of active user IDs connected to the game server, on event change like
disconnections/reconnections
simply send the updated list on each event.

### Game Events

using `NAKAMA_GAME_EVENT_URL` you must send game action
to the Nakama instance with the following body:

```json
{
  "game_id": "<game_id>",
  "action": "[READY|ERROR|STOP]",
  "message": "",
  "metadata": {}
}
```

The `action` must be one of the following:

- READY (will mark the game as ready and trigger callback event to notify players)
- ERROR (will mark the game in error and trigger callback event to notify players)
- STOP (will call Edgegap's API to stop the running deployment)

The field `message` is to provide extra data (most likely for Error Action)

The field `metadata` will be merged to the metadata of the instance info (game)

## Game Client -> Nakama (optional rpc)

TODO 

### Create Game

RPC - game_create

```json
{
  "max_players": 2,
  "user_ids": [],
  "metadata": {}
}
```

`max_players` to -1 for unlimited

if `user_ids` is empty, the user's ID calling this will be used

### Get Game

RPC - game_get

```json
{
  "game_id": "<game_id>"
}
```

### List Game

RPC - game_list

```json
{
  "query": "",
  "limit": 100,
  "cursor": ""
}
```

`query` can be used to search game with available seats

Example to list all games READY with at least 1 seat available

```json
{
  "query": "+value.metadata.edgegap.available_seats:>=1 +value.status:READY",
  "limit": 100,
  "cursor": ""
}

```

### Join Game

RPC - game_join

```json
{
  "game_id": "<game_id>",
  "user_ids": []
}
```

if `user_ids` is empty, the user's ID calling this will be used


## Matchmaker

TODO