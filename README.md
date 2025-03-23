# SwissTransfer Proxy for Resumable Uploads  

A simple proxy to enable seamless resumable uploads with [SwissTransfer](https://www.swisstransfer.com).  

## Why?  

SwissTransfer's API currently only supports chunked uploads, making it difficult to use with background `URLSession` on iOS. This proxy provides:  

- A **non-chunked upload endpoint** as a drop-in replacement for the existing API.  
- A foundation for **resumable uploads**, following [RFC Resumable Upload Specification](https://www.ietf.org/archive/id/draft-ietf-httpbis-resumable-upload-07.html).  

The next step is full resumable upload support, allowing **out-of-the-box background uploads** on iOS.  

## Getting Started  

### Server-Side Setup  

Run the proxy server (port 8080 is used):  
```sh
go run main.go
```  

### Client-Side Usage  

Upload files to this endpoint:  
```
POST /upload?containerUUID=<ContainerUUID>&uploadFileUUID=<FileUUID>
```  
Headers:  
- `x-upload-host`: The upload host for the file transfer.  

## Roadmap  

- ✅ Non-chunked upload proxy  
- ⏳ Implement resumable uploads following [RFC 9110](https://www.ietf.org/archive/id/draft-ietf-httpbis-resumable-upload-07.html)  
