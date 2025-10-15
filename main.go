package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/v62/github"
)

func main() {
	log.SetFlags(0)
	flag.Parse()
	if err := run(context.Background(), flag.Arg(0)); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, url string) error {
	if url == "" {
		return errors.New("expect github issue url as the first argument")
	}
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return errors.New("please set GITHUB_TOKEN env")
	}
	p, err := parseURL(url)
	if err != nil {
		return err
	}
	client := github.NewClient(nil).WithAuthToken(token)

	if p.isPr {
		raw, _, err := client.PullRequests.GetRaw(ctx, p.org, p.repo, p.issueNumber, github.RawOptions{Type: github.Diff})
		if err != nil {
			return err
		}
		_, err = os.Stdout.WriteString(raw)
		return err
	}

	issue, _, err := client.Issues.Get(ctx, p.org, p.repo, p.issueNumber)
	if err != nil {
		return err
	}
	var out []byte
	out = append(out, "<github_issue><title>"...)
	out = append(out, issue.GetTitle()...)
	out = append(out, "</title><date>"...)
	out = issue.GetCreatedAt().AppendFormat(out, "Monday, 02 Jan 2006")
	out = append(out, "</date>\n"...)
	out = append(out, issue.GetBody()...)
	out = maybeAddNewline(out)
	out = append(out, "</github_issue>\n"...)

	opt := &github.IssueListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100}}
	for {
		comments, resp, err := client.Issues.ListComments(ctx, p.org, p.repo, p.issueNumber, opt)
		if err != nil {
			return err
		}
		for _, comment := range comments {
			out = append(out, "<comment><user>"...)
			out = append(out, comment.User.GetLogin()...)
			out = append(out, "</user><date>"...)
			out = comment.GetCreatedAt().AppendFormat(out, "Monday, 02 Jan 2006")
			out = append(out, "</date>\n"...)
			out = append(out, comment.GetBody()...)
			out = maybeAddNewline(out)
			out = append(out, "</comment>\n"...)
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	_, err = os.Stdout.Write(out)
	return err
}

func maybeAddNewline(b []byte) []byte {
	if len(b) == 0 || b[len(b)-1] == '\n' {
		return b
	}
	return append(b, '\n')
}

type issueParams struct {
	org         string
	repo        string
	issueNumber int
	isPr        bool
}

var githubIssueURL = regexp.MustCompile(`^\Qhttps://github.com/\E([\w-]+)/([\w-]+)/(?:issues|pull)/(\d+)$`)

func parseURL(s string) (*issueParams, error) {
	if i := strings.IndexByte(s, '#'); i != -1 { // strip fragment if present
		s = s[:i]
	}
	m := githubIssueURL.FindStringSubmatch(s)
	if m == nil {
		return nil, fmt.Errorf("%q does not match %v", s, githubIssueURL)
	}
	n, err := strconv.Atoi(m[3])
	if err != nil {
		return nil, err
	}
	return &issueParams{
		org:         m[1],
		repo:        m[2],
		issueNumber: n,
		isPr:        strings.Contains(s, "/pull/"),
	}, nil
}
