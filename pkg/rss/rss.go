package rss

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"

	feedO "github.com/gorilla/feeds"
	feedI "github.com/mmcdole/gofeed"

	"github.com/nlsun/rss-reflector/pkg/log"
)

var logger = log.DefaultLogger

//Your podcast doesn’t seem to contain any episodes. Try adding an episode with this format
//<item>
//  <title>Interesting episode title</title>
//  <description>Short description about the episode</description>
//  <pubDate>Tue, 02 Oct 2016 19:45:01</pubDate>
//  <guid isPermaLink="false">insert a unique id for the episode</guid>
//  <enclosure url="http://example.com/episode1.mp3" length="5860687" type="audio/mpeg" />
//</item>

func GenYoutubeRSS(ctx context.Context, qPath, qRawQuery, dstHost, prePath string) (string, error) {
	logger.Printf("parsing: %s %s", qPath, qRawQuery)

	qUrl := url.URL{
		Scheme:   "https",
		Host:     "www.youtube.com",
		Path:     qPath,
		RawQuery: qRawQuery,
	}
	logger.Print("query uri: ", qUrl.String())

	req, err := http.NewRequest(http.MethodGet, qUrl.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Print(err)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("resp status %s", resp.Status)
	}

	inFeed, err := feedI.NewParser().Parse(resp.Body)
	if err != nil {
		return "", err
	}

	outFeed := &feedO.Feed{
		Title:       inFeed.Title,
		Link:        &feedO.Link{Href: inFeed.Link},
		Description: inFeed.Description,
		Author:      &feedO.Author{Name: inFeed.Author.Name, Email: inFeed.Author.Email},
	}
	if inFeed.UpdatedParsed != nil {
		outFeed.Updated = *inFeed.UpdatedParsed
	}
	if inFeed.PublishedParsed != nil {
		outFeed.Created = *inFeed.PublishedParsed
	}

	for _, item := range inFeed.Items {
		ytLink, err := parseYoutubeLink(item.Link, dstHost, prePath)
		if err != nil {
			return "", err
		}
		o := &feedO.Item{
			Title:       item.Title,
			Link:        &feedO.Link{Href: ytLink},
			Description: item.Description,
			Author:      &feedO.Author{Name: item.Author.Name, Email: item.Author.Email},
			Id:          item.GUID,
		}
		if item.UpdatedParsed != nil {
			o.Updated = *item.UpdatedParsed
		}
		if item.PublishedParsed != nil {
			o.Created = *item.PublishedParsed
		}

		outFeed.Items = append(outFeed.Items, o)
	}

	return outFeed.ToRss()
}

func parseYoutubeLink(link, host, prePath string) (string, error) {
	u, err := url.Parse(link)
	if err != nil {
		return "", err
	}
	u.Scheme = "http"
	u.Host = host
	u.Path = path.Join(prePath, u.Path)
	return u.String(), nil
}
