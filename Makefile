run-example:
	echo -e 'https://golang.org\nhttps://godoc.org\nhttp://google.com\nhttp://bad-url.com' | go run url_grabber.go


.PHONY: run-example
