# logma: A set of indoctrined heroku logging related event schemas

We'd ideally like to expand this to any known logged event type. Heroku Postgres, Heroku Redis, Heroku Connect ... anything that has a standard schema.

*Note: These should probably be Protobuf, Thrift, Avro, Capn Proto or whatever, so that they can deal with versions... In due time*

## You'll need...

* Go
* bmizerany/lpx
* kr/logfmt
* Patience as we figure things out

