# `goprep` - Programmable REverse Proxy

This is a fast reverse proxy in Go which lets you modify the response in a Python-like
language called [Starlark](https://github.com/bazelbuild/starlark/blob/master/spec.md).
## Use Case

Wherever you think to yourself:

> this existing API/service is amazing, if only it had feature x it would be perfect

This is where `goprep` is your friend. Just place `goprep` in front of your dream API and
write a script to modify the response to your liking.

We include an [example](examples/cors-allow-cookies/scripts/cors-cookies.star) which sets the
response header properly to make the proxied service accessible across domains.


## Usage

`goprep` is meant to be used within Docker. The simplest way to run it is somehing like:

```bash
docker run \
    -p 6350:6350\
    -e SOURCE_URL=https://httpbin.org/\
    -v $PWD/examples/cors-allow-cookies/scripts/:/etc/goprep/scripts\
    codomatech/goprep
```

It can be used with Compose or your favourite docker orchestration environment in a similar manner.

### Environment Variables

- `SOURCE_URL`: *required*. The full URL of the API/service which sits behind `goprep`.
- `PORT`: *optional*. Default=`6350`. The port which `goprep` will listen on.
- `SCRIPTS_DIR`: *optional*. Default=`'/etc/goprep/scripts'`. The directory where your scripts are located.

### Proxy Scripts

A proxy script is a script in [Starlark](https://github.com/bazelbuild/starlark) which describes how the response
should be modified.

The main function in the script is named `modify` with the following signature:
```python

def modify(req, resp):
    # req is a dict of request parameters
    # resp is a dict of response parameters
    ...
    # modify can return override values for headers and body, both are optional
    return {
        'headers': {},  # dict of header overrides
        'body': ...,    # string of body replacement
    }
```

Please check the [examples directory](examples) for more insights.

---
`goprep` is a work of :heart: by [Codoma.tech](https://www.codoma.tech/).
