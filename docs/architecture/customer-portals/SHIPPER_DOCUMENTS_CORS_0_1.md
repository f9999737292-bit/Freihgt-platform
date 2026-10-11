# Shipper documents CORS 0.1

The shipper documents UI is already in `main`. This stage only adds `X-Content-SHA256` to the gateway CORS expose allowlist so cross-origin browser script can read the attachment checksum header that the gateway already returns.

Allowed origins still receive an explicit list:

```text
Access-Control-Expose-Headers: X-Request-ID, Content-Disposition, X-Content-SHA256
```

`Content-Type` and `Content-Length` stay CORS-safelisted and are not added. Origin matching, `Vary`, methods, request headers, and preflight behavior are unchanged. Document authorization, the attachment proxy, and storage are unchanged.

```text
SHIPPER_DOCUMENTS_CORS_GATE=PASS
X_CONTENT_SHA256_GATEWAY_HEADER=YES
X_CONTENT_SHA256_CORS_EXPOSED=YES
CONTENT_DISPOSITION_CORS_EXPOSED=YES
CONTENT_LENGTH_EXPLICIT_EXPOSE_REQUIRED=NO
CONTENT_TYPE_EXPLICIT_EXPOSE_REQUIRED=NO
BUSINESS_LOGIC_CHANGED=NO
DOCUMENT_AUTH_CHANGED=NO
STORAGE_CHANGED=NO
MIGRATION_CREATED=NO
STAGING_DEPLOYED=NO
```
