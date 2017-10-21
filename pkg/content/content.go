package content

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/shlex"

	"github.com/nlsun/rss-reflector/pkg/log"
	"github.com/nlsun/rss-reflector/pkg/util"
)

type Source string

type TaskRequest struct {
	Src Source // Content source
	Uri string // Content uri
}

type internalTaskRequest struct {
	req TaskRequest     // The request
	ctx context.Context // A context that is only briefly passed across a channel
}

type taskResponse struct {
	path string // Path to file to serve
	err  error  // Errors encountered
}

type fetcherTask struct {
	req       TaskRequest
	respC     chan<- taskResponse
	finC      <-chan struct{}
	tmpdir    string
	datadir   string
	ytdl      string
	ytdlFlags string
	maxndf    int
}

type Fetcher struct {
	tmpdir    string                   // Directory to store temporary data
	datadir   string                   // Directory to store data
	ytdl      string                   // Path to youtube-dl
	ytdlFlags string                   // youtube-dl command line flags
	maxndf    int                      // Max number of data files to cache
	reqQueue  chan internalTaskRequest // The client request queue
	respQueue chan taskResponse        // The handler response queue
	finQueue  chan struct{}            // The client fin response queue
}

const (
	YoutubeSource Source = "youtube"
)

var logger = log.DefaultLogger

func (s Source) String() string {
	return string(s)
}

func NewFetcher(basedir, ytdl, ytdlFlags string, maxndf int) (*Fetcher, error) {
	if err := exec.Command(ytdl, "--version").Run(); err != nil {
		return nil, err
	}

	datadir := filepath.Join(basedir, "data")
	tmpdir := filepath.Join(basedir, "tmp")

	if err := os.MkdirAll(datadir, util.DefaultDirPerm); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(tmpdir, util.DefaultDirPerm); err != nil {
		return nil, err
	}

	fetcher := &Fetcher{
		datadir:   datadir,
		tmpdir:    tmpdir,
		ytdl:      ytdl,
		ytdlFlags: ytdlFlags,
		maxndf:    maxndf,

		// The queue purposefully has no buffering so we will never attempt
		// to fetch things concurrently. This is because we expect this to
		// run on weak servers.
		reqQueue:  make(chan internalTaskRequest),
		respQueue: make(chan taskResponse),
		finQueue:  make(chan struct{}),
	}

	go fetcher.handleTasks()

	return fetcher, nil
}

func (f *Fetcher) handleTasks() {
	for intreq := range f.reqQueue {
		logger.Printf("fetcher handling task %+v", intreq.req)
		t := fetcherTask{
			req:       intreq.req,
			respC:     f.respQueue,
			finC:      f.finQueue,
			tmpdir:    f.tmpdir,
			datadir:   f.datadir,
			ytdl:      f.ytdl,
			ytdlFlags: f.ytdlFlags,
			maxndf:    f.maxndf,
		}
		t.doTask(intreq.ctx)
		logger.Printf("fetcher completed task %+v", intreq.req)
	}
	logger.Print("request queue closed, terminating task handler")
}

func (f fetcherTask) doTask(ctx context.Context) {
	path, err := f.doTaskHelper(ctx)
	f.respC <- taskResponse{path: path, err: err}
	<-f.finC
}

// Downloads to a temporary location and then moves it to the final location
// after the download completes. This is so we don't accidentally use
// half-finished downloads.
func (f fetcherTask) doTaskHelper(ctx context.Context) (string, error) {
	// Even if the context closes, we still want to complete the download. That
	// way it'll be cached when the request retries.

	// youtube-dl does this weird thing where you have to use it's file name
	// templates so you cannot use exact string match.

	u, err := url.Parse(f.req.Uri)
	if err != nil {
		return "", err
	}
	fnamePrefix := strings.Replace(u.RequestURI(), "_", "__", -1)
	fnamePrefix = strings.Replace(fnamePrefix, "/", "_", -1)
	fnamePrefix = f.req.Src.String() + "_" + fnamePrefix
	// tmpfPrefix is only a prefix because it is created by youtube-dl
	tmpfPrefix := filepath.Join(f.tmpdir, fnamePrefix)
	// dataf is not a prefix because it is the file we name
	dataf := filepath.Join(f.datadir, fnamePrefix)

	if f.req.Src != YoutubeSource {
		return "", fmt.Errorf("source %s unimplemented", f.req.Src)
	}

	if ok, err := util.FileExists(dataf); err != nil {
		return "", err
	} else if ok {
		return dataf, nil
	}

	// Wipe the tmp file location first in case there was stale data.
	if tmpf, err := util.FindFileWithPrefix(tmpfPrefix); err != nil {
		return "", err
	} else if tmpf != "" {
		logger.Print("removing stale tmp file %s", tmpf)
		if err := os.RemoveAll(tmpf); err != nil {
			return "", err
		}
	}

	if files, err := util.FilesSortedByOldest(f.datadir); err != nil {
		return "", err
	} else if len(files) >= f.maxndf {
		logger.Printf("removing files, count %d max %d", len(files), f.maxndf)
		for _, file := range files[:len(files)-f.maxndf+1] {
			if err := os.RemoveAll(file); err != nil {
				return "", err
			}
			logger.Printf("removing cached file: %s", file)
		}
	}

	splitFlags, err := shlex.Split(f.ytdlFlags)
	if err != nil {
		return "", err
	}
	// youtube-dl forces you to use their template format if you are re-encoding
	cmdFlags := append(splitFlags, "--output", tmpfPrefix+`.%(ext)s`, f.req.Uri)
	logger.Printf("%s %+v", f.ytdl, cmdFlags)
	cmd := exec.Command(f.ytdl, cmdFlags...)
	outputB, err := cmd.CombinedOutput()
	if err != nil {
		msg := err.Error()
		if outputB != nil {
			msg += "\n" + string(outputB)
		}
		return "", fmt.Errorf(msg)
	}
	logger.Print(string(outputB))

	tmpf, err := util.FindFileWithPrefix(tmpfPrefix)
	if err != nil {
		return "", err
	}
	if tmpf == "" {
		return "", fmt.Errorf("tmp file with prefix %s not found", tmpfPrefix)
	}
	logger.Printf("moving %s to %s", tmpf, dataf)
	if err := os.Rename(tmpf, dataf); err != nil {
		return "", err
	}

	return dataf, nil
}

// There should never be more than one task running at a time.
//
// FinishTask MUST be called WHETHER OR NOT this succeeds.
func (f *Fetcher) SubmitTask(ctx context.Context, req TaskRequest) (string, error) {
	select {
	case f.reqQueue <- internalTaskRequest{req: req, ctx: ctx}:
		resp := <-f.respQueue
		return resp.path, resp.err
	case <-ctx.Done():
		return "", fmt.Errorf("context done before task submitted")
	}
}

// This must be called after the returned resources are no longer used. This
// allows the next task to begin.
func (f *Fetcher) FinishTask() {
	f.finQueue <- struct{}{}
}
