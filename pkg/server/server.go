package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nlsun/rss-reflector/pkg/content"
	"github.com/nlsun/rss-reflector/pkg/log"
	"github.com/nlsun/rss-reflector/pkg/rss"
	"github.com/nlsun/rss-reflector/pkg/util"
)

var logger = log.DefaultLogger

type State struct {
	addr    string           // Address to listen on
	fetcher *content.Fetcher // Content fetcher
}

const (
	rssPath     string = "/rss"
	contentPath string = "/content"

	rssPathSlash     string = rssPath + "/"
	contentPathSlash string = contentPath + "/"

	ytPrefix string = "youtube/" // Youtube prefix
)

func NewServer(addr, datadir, ytdl, ytdlFlags string, maxndf int) (*State, error) {
	logger.Println("addr", addr)
	logger.Println("data", datadir)
	logger.Println("youtube-dl", ytdl)
	logger.Println("youtube-dl flags", ytdlFlags)
	logger.Println("max num data files", maxndf)

	fetcherdir := filepath.Join(datadir, "fetcher")
	if err := os.MkdirAll(fetcherdir, util.DefaultDirPerm); err != nil {
		return nil, err
	}

	fetcher, err := content.NewFetcher(fetcherdir, ytdl, ytdlFlags, maxndf)
	if err != nil {
		return nil, err
	}

	return &State{
		addr:    addr,
		fetcher: fetcher,
	}, nil
}

func (s *State) Run() error {
	http.HandleFunc("/", s.handleDefault)
	http.HandleFunc(rssPathSlash, s.handleRSS)
	http.HandleFunc(contentPathSlash, s.handleContent)

	logger.Printf("listening on %s", s.addr)
	return http.ListenAndServe(s.addr, nil)
}

func (s *State) handleDefault(w http.ResponseWriter, r *http.Request) {
	s.handleError(w, r, http.StatusNotFound)
}

func (s *State) handleError(w http.ResponseWriter, r *http.Request, status int) {
	w.WriteHeader(status)

	switch status {
	case http.StatusUnauthorized:
		fmt.Fprint(w, "401 rss-reflector unauthorized")
	case http.StatusNotFound:
		fmt.Fprint(w, "404 rss-reflector not found")
	case http.StatusInternalServerError:
		fmt.Fprint(w, "500 rss-reflector internal server error")
	}
}

func (s *State) handleRSS(w http.ResponseWriter, r *http.Request) {
	qPath := strings.TrimPrefix(r.URL.Path, rssPathSlash)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go handleEvents(ctx, cancel, w.(http.CloseNotifier), "handleRSS")

	reqHost := r.Host
	logger.Printf("handleRSS request from host %s", reqHost)
	logger.Printf("handleRSS request %+v", r)
	if strings.HasPrefix(qPath, ytPrefix) {
		reqPrePath := path.Join(contentPath, ytPrefix)
		rssStr, err := rss.GenYoutubeRSS(ctx, strings.TrimPrefix(qPath, ytPrefix), r.URL.RawQuery, reqHost, reqPrePath)
		if err != nil {
			logger.Print(err)
			s.handleError(w, r, http.StatusInternalServerError)
			return
		}
		if _, err := w.Write([]byte(rssStr)); err != nil {
			logger.Print(err.Error())
		}
	} else {
		s.handleError(w, r, http.StatusNotFound)
	}
}

func handleEvents(ctx context.Context, cancel context.CancelFunc, w http.CloseNotifier, tag string) {
	select {
	case <-w.CloseNotify():
		logger.Printf("(%s) client prematurely closed request", tag)
	case <-ctx.Done():
		// noop, cancel is called later
	}
	cancel()
	logger.Printf("(%s) event handler exited", tag)
}

func (s *State) handleContent(w http.ResponseWriter, r *http.Request) {
	qPath := strings.TrimPrefix(r.URL.Path, contentPathSlash)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go handleEvents(ctx, cancel, w.(http.CloseNotifier), "handleContent")

	if strings.HasPrefix(qPath, ytPrefix) {
		qUrl := url.URL{
			Scheme:   "https",
			Host:     "www.youtube.com",
			Path:     strings.TrimPrefix(qPath, ytPrefix),
			RawQuery: r.URL.RawQuery,
		}
		path, err := s.fetcher.SubmitTask(ctx, content.TaskRequest{
			Src: content.YoutubeSource,
			Uri: qUrl.String(),
		})
		defer s.fetcher.FinishTask()
		if err != nil {
			logger.Print(err)
			s.handleError(w, r, http.StatusInternalServerError)
			return
		}

		http.ServeFile(w, r, path)
	} else {
		s.handleError(w, r, http.StatusNotFound)
	}
}
