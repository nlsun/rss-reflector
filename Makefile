.PHONY: clean compile web-dev server-dev

SERVERDIR = pkg/cmd/rss-reflector
SERVERBIN = rss-reflector

clean:
	rm -f ${SERVERDIR}/${SERVERBIN}

compile-server:
	cd ${SERVERDIR} && \
		go build -o ${SERVERBIN} .

dev: compile-server
	# curl http://localhost:3322/rss/youtube/feeds/videos.xml?channel_id=UC9nnWZ9kRiNZ6d5UwF-sNKQ
	mkdir -p /tmp/${SERVERBIN}
	# We purposefully do not clear the tmp dir, this is because we need to
	# handle cases with old data
	./${SERVERDIR}/${SERVERBIN} --data /tmp/${SERVERBIN}
