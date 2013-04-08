package tweet

import (
	"../"
	"code.google.com/p/go.net/html"
	"fmt"
	"github.com/fluffle/goevent/event"
	irc "github.com/fluffle/goirc/client"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type Tweet struct {
	Username  string
	Fullname  string
	Tweet     string
	Timestamp string
}

func (t Tweet) String() string {
	return fmt.Sprintf("%s (%s) at %s: %s", t.Fullname, t.Username, t.Timestamp, t.Tweet)
}

func (t Tweet) Valid() bool {
	return t.Username != "" && t.Fullname != "" && t.Tweet != "" && t.Timestamp != ""
}

func init() {
	plugin.RegisterSetup(setupTweet)
}

func setupTweet(conn *irc.Conn, er event.EventRegistry) error {
	er.AddHandler(event.NewHandler(func(args ...interface{}) {
		conn, line, urlStr := args[0].(*irc.Conn), args[1].(*irc.Line), args[2].(string)
		u, err := url.Parse(urlStr)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		if u.Scheme == "http" || u.Scheme == "https" {
			if u.Host == "twitter.com" || u.Host == "www.twitter.com" {
				if strings.Contains(u.Path, "/status/") {
					go processTweetURL(conn, line, urlStr)
				}
			}
		}
	}), "URL")
	return nil
}

func processTweetURL(conn *irc.Conn, line *irc.Line, url string) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Fprintln(os.Stderr, "tweet: unexpected response:", resp)
		return
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	var tweet Tweet
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			classes := classMap(n)
			if classes["tweet-text"] {
				tweet.Tweet = nodeString(n)
			} else if classes["tweet-timestamp"] {
				tweet.Timestamp = nodeAttr(n, "title")
			} else if classes["original-tweet"] {
				tweet.Fullname = nodeAttr(n, "data-name")
				username := nodeAttr(n, "data-screen-name")
				if username != "" {
					tweet.Username = "@" + username
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	if tweet.Valid() {
		dst := line.Args[0]
		fmt.Printf("--> NOTICE[%s] %s\n", dst, tweet.String())
		conn.Notice(dst, tweet.String())
	} else {
		fmt.Println("Could not find tweet in page", url)
	}
}

func nodeAttr(node *html.Node, attr string) string {
	if node.Type == html.ElementNode {
		for _, at := range node.Attr {
			if at.Namespace == "" && at.Key == attr {
				return at.Val
			}
		}
	}
	return ""
}

func classMap(node *html.Node) map[string]bool {
	if node.Type == html.ElementNode {
		classes := strings.Split(nodeAttr(node, "class"), " ")
		results := make(map[string]bool)
		for _, class := range classes {
			if class != "" {
				results[class] = true
			}
		}
		return results
	}
	return nil
}

func nodeString(node *html.Node) string {
	switch node.Type {
	case html.TextNode:
		return node.Data
	case html.DocumentNode:
		fallthrough
	case html.ElementNode:
		var result string
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			result = result + nodeString(c)
		}
		return result
	}
	return ""
}