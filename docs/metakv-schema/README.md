
This is the proposed configuration schema for SG Mercury.

It's split up into two parts:

* [metakv-config.json](metakv-config.json) contains the structure that will be stored at the MetaKV level.  Things that refer to other `.json` files will be stored as opaque `[]byte` slices.
* metakv-*.json shows the structure of the configuration below the MetaKV level.


Currently these are just raw json files, however the idea is to convert them to json schema files, so that things like the following could be added:

- Which fields are required vs optional
- Default values
- Field level documentation

It's possible to generate sample json files from the schema using a tool like [json-schema-faker](https://github.com/json-schema-faker/json-schema-faker).

