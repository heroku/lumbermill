```
heroku create -b https://github.com/kr/heroku-buildpack-go.git <name>
heroku config:set INFLUXDB_HOST=".." \
                  INFLUXDB_USER=".." \
                  INFLUXDB_PWD=".." \
                  INFLUXDB_NAME=".." \
                  INFLUXDB_SKIP_VERIFY=true


git push heroku master
```
