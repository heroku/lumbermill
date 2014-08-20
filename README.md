# Lumbermill

This is a Go app which takes Heroku Log drains and parses the router and dyno information, and then pushes metrics to influxdb.

## Setup
### Setup Influx

Create a db, user and password, and write the details + hostname and port down.

### Deploy to Heroku

[![Deploy to Heroku](https://www.herokucdn.com/deploy/button.png)](https://heroku.com/deploy)

### Add the drain to an app

```
heroku drains:add https://<lumbermill_app>.herokuapp.com/drain --app <the-app-to-mill-for>
```

You'll then start getting metrics in your influxdb host!