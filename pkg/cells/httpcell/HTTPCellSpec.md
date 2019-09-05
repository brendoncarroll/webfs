# HTTPCell Spec

This is probably the most simple networked cell implementation.
It provides no security from the server snooping on the contents, and no security from the server returning illegitimate data.
That all said, it is useful for testing, and may be useful for organizations with an existing permissions system, or who want to give public read access.

The Get operation is a simple HTTP `GET` request to the url.  The response body will be the current value.

The CAS operation is a HTTP `PUT` to the url.
It includes a header `X-Current` which is the Base64 URL-encoded SHA3-256 hash of the believed current value.
The response body will be the current value.
If the CAS was successful the returned data will equal the proposed data.

## Reference Server
This section applies to only the reference server

The server starts with no cells. To create one `POST` to it's name.
```
curl -X POST http://mycellserver/myNewCell
```

Now there is an empty cell.  HttpCells are uniquely identified by their URL.