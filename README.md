# Lumbermill

This is a Go app which takes Heroku Log drains and parses the router and dyno information, and then pushes metrics to influxdb.

## Setup
### Setup Influx

Create a db, user and password, and write the details + hostname and port down.

### Install on Heroku
```
heroku create -b https://github.com/kr/heroku-buildpack-go.git <lumbermill_app>
heroku config:set INFLUXDB_HOSTS="<host:port>" \
                  INFLUXDB_USER="<user>" \
                  INFLUXDB_PWD="<password>" \
                  INFLUXDB_NAME="<dbname>" \
                  INFLUXDB_SKIP_VERIFY=true


git push heroku master

heroku drains:add https://<lumbermill_app>.herokuapp.com/drain --app <the-app-to-mill-for>
```

You'll then start getting metrics in your influxdb host!