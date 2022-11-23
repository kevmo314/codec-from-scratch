# Video Encoding from Scratch

Ever wondered how a video encoder works? This is a simple video encoder
that walks through building a video encoder from scratch to achieve a 90% compression ratio!

The documentation is in `main.go`. You can run this code by running
`cat video.rgb24 | go run main.go` and you should see this as output

```sh
$ cat video.rgb24 | go run main.go
2022/11/23 13:54:03 Raw size: 53996544 bytes
2022/11/23 13:54:03 YUV420P size: 26998272 bytes (50.00% original size)
2022/11/23 13:54:03 RLE size: 13592946 bytes (25.17% original size)
2022/11/23 13:54:15 DEFLATE size: 5457415 bytes (10.11% original size)
```

The actual encoding is done in about 120 lines of code. This is meant
to be a didactic exercise rather than a comprehensive guide, but maybe
if there's interest we could add more features that appear in modern video
codecs.