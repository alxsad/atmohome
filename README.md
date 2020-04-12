# Atmohome - telegram bot that measures temperature and humidity written in golang

### Rapbserry Pi crontab -e
```
* * * * * python3 /home/pi/dht22.py
```

### Build telegram bot for ARM arch
```bash
$ go mod download
$ GOOS=linux GOARCH=arm GOARM=5 go build .
```

### Bot commands
* /last - presents last measurement
* /day - render graph with measurements during last 24 hours

### Example
![](output.png)