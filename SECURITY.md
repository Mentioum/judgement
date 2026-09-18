# Security

Do not include credentials or private evaluation data in public issues.
Report suspected vulnerabilities through this repository's GitHub private
vulnerability reporting feature.

The current release line receives fixes. Store credentials in environment
variables or your existing secret manager. Custom API roots receive the bearer
token and must be trusted. The CLI refuses redirects and non-loopback plain HTTP.
Remote error response bodies are intentionally omitted from diagnostics.

Input and successful output can contain private data. This tool does not encrypt
files you redirect to disk, scrub model answers, or control retention by the API
provider. Review the provider's terms for the data you process.
