# lpxgen: Random Logplex Batches for testing.

## Example usage

Let's say you want to test out
[lumbermill](https://github.com/heroku/lumbermill). You can use lpxgen
to send random logplex batches at it with different types of log lines
in the batches:

```bash
$ lpxgen -count 10000 -min 10 -max 100 -dist router:.8,dynomem:.1,dynoload:.1 http://lumbermill/drain
```

This will fire off 10000 batches with a batch size between 10 and 100,
and ensuring that 80% of messages sent are Heroku router messages,
while the other 20% are split between dynomem and dynoload.


## Contributing and Feedback

If you'd like to fix or contribute something, please fork and submit a pull
request, or open an issue. There's lots of room for improvement, and much
more work to be done.

## Authors

Andrew Gwozdziewycz <web@apgwoz.com>

## Copyright

Copyright 2014, Andrew Gwozdziewycz, <web@apgwoz.com>

Licensed under the GNU GPLv3. See LICENSE for more details
