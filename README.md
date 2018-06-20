# CloudEvent Verify

A tool to help verify CloudEvents according to the <a href="https://github.com/cloudevents/spec/blob/master/spec.md">specifications</a>.

## Usage

If no value is returned, the CloudEvent is correct. Otherwise, an error will be returned.

If no arguments are given, a server on port 80 will be started.
- To see how to use the server, see the <a href="https://github.com/cloudevents/spec/blob/master/http-transport-binding.md">HTTP Transport Binding for CloudEvents</a>.

### Arguments (Optional)

- `f` - File to verify
	- File path to a CloudEvent in JSON
	- Use `-` to read from `stdin`
- `p` - Server port (default 80)
- `crt` - File path to certificate for TLS
- `key` - File path to key for TLS