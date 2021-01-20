# BCHD REST JSON Schema

We use [JSON Schema](https://json-schema.org/) to validate our responses
during tests with [gojsonschema](https://github.com/xeipuuv/gojsonschema).

The format of those JSON files is based on the auto-generated [bchrpc_swagger.json](../../web/bchrpc.swagger.json)
but with more restrictions because we expect 1 specific response (constant tokenID etc...).

[JSON Schema specification](https://json-schema.org/understanding-json-schema/index.html)