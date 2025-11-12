# Jellyfin-RandomMovie

Simple web app for find a random movie from your Jellyfin Server.

![Homepage](./media/homepage.png)

## Installation

### Docker pull

```bash
docker pull cedev/random-movie-jellyfin:v1
docker run -d -p 8080:8080 --name jellyfin-random cedev/random-movie-jellyfin:v1
```

Decommente the `image` line in `docker-compose.yml` file if you want to use this method.

### Manual build

```bash
git clone https://github.com/cedev-1/Jellyfin-RandomMovie.git
cd Jellyfin-RandomMovie
docker-compose up -d --build
```

## Debug
```bash
docker-compose logs -f
```

![logs](./media/logs.png)

## Stop
```bash
docker-compose down
```

## More media
![configuration-page](./media/configuration-page.png)
![user-page](./media/user-page.png)
![hoverview](./media/hoverview.png)