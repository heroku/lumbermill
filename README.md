# DEPRECATED: See [the blog post](https://engineering.heroku.com/blogs/2016-05-26-heroku-metrics-there-and-back-again/) for more information.


[![Travis](https://img.shields.io/travis/heroku/lumbermill.svg)](https://travis-ci.org/heroku/lumbermill)
[![GoDoc](https://godoc.org/github.com/heroku/lumbermill?status.svg)](http://godoc.org/github.com/heroku/lumbermill)

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

### Environment Variables

* `CRED_STORE`: `user1:pass1|user2:pass2|userN:passN` -- Basic Auth credentials for HTTP endpoints.
* `DEBUG`: Turn on debug mode
* `INFLUXDB_USER`: User that has permissions to write to the database
* `INFLUXDB_PWD`: Password for the user
* `INFLUXDB_NAME`: Database name in InfluxDB
* `INFLUXDB_HOSTS`: InfluxDB hosts in the hash ring.
* `INFLUXDB_SKIP_VERIFY`: Skip TLK verification?
* `LIBRATO_TOKEN`: Librato token for posting metrics to
* `LIBRATO_OWNER`: User that owns said token
* `LIBRATO_SOURCE`: Source for Librato metrics.
* `PORT`: 
