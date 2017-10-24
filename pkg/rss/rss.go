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
		Title:       revStr(inFeed.Title),
		Link:        &feedO.Link{Href: inFeed.Link},
		Description: revStr(inFeed.Description),
		Author:      &feedO.Author{Name: revStr(inFeed.Author.Name), Email: revStr(inFeed.Author.Email)},
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
			Title: revStr(item.Title),
			// This Link is not used in the final XML, it's just used to
			// pass information to the next parsing stage.
			Link:        &feedO.Link{Href: ytLink},
			Description: revStr(item.Description),
			Author:      &feedO.Author{Name: revStr(item.Author.Name), Email: revStr(item.Author.Email)},
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

	// The reason this dance is necessary is because gorilla/feeds has a bug
	// where it does not internally convert the Link to an Enclosure.
	rssThing := &feedO.Rss{outFeed}
	finalRssFeed := rssThing.RssFeed()
	for i := range finalRssFeed.Items {
		finalRssFeed.Items[i].Enclosure = &feedO.RssEnclosure{
			// A possible issue is that we leave the `Length` blank.
			// We do this because we don't actually know anything about the
			// contents of the link.
			// It seems, however, that rss feed readers are generally ok with
			// this.
			Url: finalRssFeed.Items[i].Link,
			// XXX Type is current hardcoded, we should be able to configure
			// these through special query parameters in the url, since
			// we construct these urls.
			Type: "audio/mpeg",
		}
		// Clear out the unused Link
		finalRssFeed.Items[i].Link = ""
	}

	return feedO.ToXML(finalRssFeed)
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

func revStr(input string) string {
	// https://stackoverflow.com/questions/1752414/how-to-reverse-a-string-in-go/1754209#1754209
	// https://groups.google.com/forum/#!topic/golang-nuts/oPuBaYJ17t4

	// Get Unicode code points.
	n := 0
	rune := make([]rune, len(input))
	for _, r := range input {
		rune[n] = r
		n++
	}
	rune = rune[0:n]
	// Reverse
	for i := 0; i < n/2; i++ {
		rune[i], rune[n-1-i] = rune[n-1-i], rune[i]
	}
	// Convert back to UTF-8.
	return string(rune)
}
