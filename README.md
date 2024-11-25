# beoutil

**beoutil** is a command-line utility for controlling Bang & Olufsen products that support BeoLink Multiroom via the
beoremote API. It includes support for multiroom playback, power management, queue control, notifications, and
integration with Deezer for music playback.

## Features

- Discover and control B&O products supporting BeoLink Multiroom via the beoremote API.
- Manage power states, streams, queues and sources across multiple devices.
- Deezer integration: search for artists, albums, and tracks, and queue them for playback via the public Deezer API.
- Monitor product notifications and manage timers.

## Installation

To install **beoutil**, make sure you have [Go installed](https://golang.org/dl/) and run the following command:

```bash
go install github.com/andy-js/beoutil@latest
```

Alternatively, clone the repository and install it from the source:

```bash
git clone https://github.com/andy-js/beoutil.git
cd beoutil
go install
```

## Usage

The basic usage of **beoutil** is as follows:

```bash
beoutil [global options] command [command options]
```

### Commands

#### Product Discovery & Control

- `find-products`: Discover products using MDNS.
- `list-products`: List discovered products.

#### Multiroom Control

- `add-listener`: Add a listener to the primary experience.
- `remove-listener`: Remove a listener from the primary experience.

#### Deezer Integration

- `search-artist`: Search for an artist on Deezer.
- `list-albums`: List albums by a specific artist.
- `list-tracks`: List tracks on a specific album.
- `queue-track`: Queue a track from Deezer on a specific B&O product.
- `queue-album`: Queue an album from Deezer on a specific B&O product.

#### Notifications

- `watch`: Watch notifications from a product.

#### Power Management

- `all-standby`: Put all products into standby mode.
- `standby`: Put a specific product into standby mode.
- `poweron`: Power on a product.
- `reboot`: Reboot a product.

#### Queue Management

- `get-queue`: Get the current playback queue.
- `clear-queue`: Clear the current playback queue.
- `remove-qitem`: Remove an item from the playback queue.
- `move-qitem`: Move an item within the playback queue.
- `play-qitem`: Play an item from the playback queue.
- `set-repeat`: Set the queue to repeat mode.
- `set-random`: Set the queue to random mode.

#### Source Selection

- `get-sources`: Get sources available to a product.
- `get-active`: Get the currently active sources for a product.
- `set-active`: Set the active source on a product.
- `clear-active`: Clear active source.

#### Speaker Control

- `get-volume`: Get the speaker volume level.
- `set-volume`: Set the speaker volume level.
- `get-muted`: Check if the speaker is muted.
- `set-muted`: Mute or unmute the speaker.

#### Stream Control

- `pause`: Pause the current stream.
- `play`: Unpause the current stream.
- `forward`: Play the next track.
- `backward`: Play the previous track.
- `stop`: Stop the stream.

#### Timer Management

- `get-timers`: Get the list of timers from a product.
- `delete-timer`: Delete a specific timer.

To see the usage for each command run:

```bash
beoutil help [command]
```

## Examples

### Discover Products on the network

The `find-products` command uses MDNS to discover products on the network. It stores basic information
about the products in a cache file in the user's home directory.

```bash
beoutil find-products
```

Output:
```plaintext
Scanning for products...
Found 3 products.
```

NOTE: Not every product on the network will respond. Products such as the BeoLiving Intelligence and
Mozart based speakers will not respond because they don't implement the BeoRemote API. They are however
visible to products on the network that do, and may still show up in the output of some commands.

### List Known Products

The **list-products** command asks each product in the cache file about its current state, and any products
on the network it may know about. The responses are stitched together to provide a complete list of the
products on the network. This can be used to obtain a product's IP or JID, which may be required when
interacting with a product.

```bash
beoutil list-products
```
Output:
```plaintext
NAME            ROLE    IP           JID                                              ONLINE  STATE
Beosound 2      -       192.168.0.17 6658.1665811.27297491@products.bang-olufsen.com  true    play
bli             -       -            1704.11170466.18390529@products.bang-olufsen.com true    -
BeoSound Emerge -       -            2738.1273701.36226734@products.bang-olufsen.com  true    -
Beosound 1      -       192.168.0.94 6655.1665511.26582735@products.bang-olufsen.com  true    play
Beosound Stage  -       192.168.0.62 4541.1200463.35784501@products.bang-olufsen.com  true    -
```

NOTE: The IPs of bli and BeoSound Emerge are not listed because they are not present in the cache file.

### Get Sources available to a Product

The **get-sources** command can be used to retrieve a list of all sources available to a product. This can be
used to obtain a source's ID, which is required when setting a product's active source. In this example we ask
BeoSound 1 for its available sources.

```bash
beoutil get-sources 192.168.0.94
```
Output:
```plaintext
PRODUCT NAME    SOURCE NAME                 SOURCE ID                                                                       LINKABLE
BeoSound Emerge B&O Radio                   radio:2738.1273701.36226734@products.bang-olufsen.com                           true
BeoSound Emerge HomeMedia                   dlna:2738.1273701.36226734@products.bang-olufsen.com                            false
Beosound 1      AirPlay                     airplay:6655.1665511.26582735@products.bang-olufsen.com                         false
Beosound 1      Deezer                      deezer:6655.1665511.26582735@products.bang-olufsen.com                          true
```

NOTE: The output here has been shortened for brevity. The actual output is quite long, as each B&O product
maintains a list of not just its local sources, but the sources of every product on the network.

### Borrow Source from another Product

The **set-active** command can be used to play a local or remote source. In this example we ask BeoSound 2 to play
(borrow) BeoSound 1's Deezer Queue.

```bash
beoutil set-active 192.168.0.17 deezer:6655.1665511.26582735@products.bang-olufsen.com
```

### Expand a multiroom experience

The **add-listener** command can be used to join one product to a source currently playing on another product. In
this example we ask BeoSound 1 to add BeoSound 2 as a listener.

```bash
beoutil add-listener 192.168.0.94 6658.1665811.27297491@products.bang-olufsen.com
```

### Search for an Artist on Deezer

The **search-artist** command can be used to look up the ID of an artist on Deezer. The artist ID is required
to list an artist's albums.

```bash
beoutil search-artist "Lacuna Coil" --limit 5
```
Output:
```plaintext
ID        NAME
2219      Lacuna Coil
4567231   Cristina Scabbia
11939847  Penumbra
12534782  Moris Blak
65528442  Kassogtha
```

### List Available Albums for an Artist on Deezer

The **list-albums** command can be used to look up the ID of an artist's album of Deezer.

```bash
beoutil list-albums 2219
```
Output:
```plaintext
ID        TITLE                                                    TYPE    EXPLICIT RELEASED
330058527 Comalies XX                                              album   false    2022-10-14
219520932 Live From The Apocalypse                                 album   false    2021-06-25
103993442 Black Anima (Bonus Tracks Version)                       album   false    2019-10-11
13200470  Delirium                                                 album   true     2016-05-27
```

### Queue an Album from Deezer to play on a Product

The **queue-album** command can be used to play an album from Deezer. In this example the entire album is added to
the end of the queue.

```bash
beoutil queue-album 192.168.0.94 219520932
```

### List Tracks on an Album on Deezer

The **list-tracks** command can be used to look up the ID of an album's track on Deezer.

```bash
beoutil list-tracks 11320924
```
Output:
```plaintext
ID        TITLE
108572702 Fragile
108572704 To the Edge
108572706 Our Truth
108572708 Within Me
108572710 Devoted
```

### Play a track from Deezer on a Product

The **queue-track** command can be used to play a track from Deezer. In this example we pass `--play now` so that
playback begins immediately. This also clears the current play queue.

```bash
beoutil queue-track --play now 192.168.0.94 108572702
```

### Get the play queue from a product

The **get-queue** command can be used to obtain a product's play queue. Each product has a play queue that is
maintained as long as the product has power. In this example we ask BeoSound 2 for its play queue.

```bash
beoutil get-queue 192.168.0.17
```

Output:
```plaintext
PTR     PLID    TRACK                    ARTIST
        11949   Stinkfist                TOOL
        11950   Eulogy                   TOOL
        11951   H.                       TOOL
        11952   Useful Idiot             TOOL
------> 11953   Forty Six & 2            TOOL
        11954   Message To Harry Manback TOOL
        11955   Hooker With A Penis      TOOL
        11956   Intermission             TOOL
        11957   Jimmy                    TOOL
        11958   Die Eier von Satan       TOOL
        11959   Pushit                   TOOL
        11960   Cesaro Summability       TOOL
        11961   Ænema                    TOOL
        11962   (-) Ions                 TOOL
        11963   Third Eye                TOOL
Repeat: off	Random: on
```

## Contributing

Feel free to open issues or submit pull requests if you'd like to contribute to the project.

## License

This project is licensed under the MIT License. See the `LICENSE` file for more details.
