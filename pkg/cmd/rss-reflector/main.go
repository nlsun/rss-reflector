package main

import (
	"flag"

	"github.com/nlsun/rss-reflector/pkg/log"
	"github.com/nlsun/rss-reflector/pkg/server"
)

var logger = log.DefaultLogger

func main() {
	var addr string
	var datadir string
	var ytdl string
	var maxNumDataFiles int
	var ytdlFlags string

	flag.StringVar(&addr, "addr", ":3322", "Address to listen on")
	flag.StringVar(&datadir, "data", "data", "Data directory")
	flag.StringVar(&ytdl, "youtube-dl", "youtube-dl", "youtube-dl")
	flag.IntVar(&maxNumDataFiles, "max-data-count", 20, "Max number of cached data files")
	defaulYtdlFlags := `--extract-audio --audio-format mp3 --postprocessor-args "-strict experimental"`
	flag.StringVar(&ytdlFlags, "youtube-dl-flags", defaulYtdlFlags, "youtube-dl flags")

	flag.Parse()

	sv, err := server.NewServer(addr, datadir, ytdl, ytdlFlags, maxNumDataFiles)
	if err != nil {
		logger.Fatal("new server error: ", err)
	}
	logger.Fatal("server run error: ", sv.Run())
}
