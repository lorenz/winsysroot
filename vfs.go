package main

import (
	"fmt"
	"strings"
)

type RedirectingWith string

const (
	RedirectingWithDefault      RedirectingWith = ""
	RedirectingWithFallthrough  RedirectingWith = "fallthrough"
	RedirectingWithFallback     RedirectingWith = "fallback"
	RedirectingWithRedirectOnly RedirectingWith = "redirect-only"
)

type VFS struct {
	Version          int             `json:"version"`
	CaseSensitive    *bool           `json:"case-sensitive,omitempty"`
	UseExternalNames *bool           `json:"use-external-names,omitempty"`
	OverlayRelative  *bool           `json:"overlay-relative,omitempty"`
	RedirectingWith  RedirectingWith `json:"redirecting-with,omitempty"`
	Roots            []*Inode        `json:"roots"`
}

type Inode struct {
	Type             string   `json:"type"`
	Name             string   `json:"name"`
	UseExternalName  *bool    `json:"use-external-name,omitempty"`
	ExternalContents string   `json:"external-contents,omitempty"`
	Contents         []*Inode `json:"contents,omitempty"`
}

func (r *Inode) Place(dir string, caseSensitive bool, i *Inode) error {
	dirParts := strings.Split(dir, "/")
	return r.place(dirParts, caseSensitive, i)
}

func (r *Inode) place(dir []string, caseSensitive bool, i *Inode) error {
	if len(dir) == 0 {
		r.Contents = append(r.Contents, i)
		return nil
	}
	if r.Type != "directory" {
		return fmt.Errorf("failed placing inode, %q not a directory", r.Name)
	}
	for _, sub := range r.Contents {
		if caseSensitive && strings.EqualFold(sub.Name, dir[0]) || !caseSensitive && sub.Name == dir[0] {
			return sub.place(dir[1:], caseSensitive, i)
		}
	}
	newI := Inode{
		Type: "directory",
		Name: dir[0],
	}
	if err := newI.place(dir[1:], caseSensitive, i); err != nil {
		return err
	}
	r.Contents = append(r.Contents, &newI)
	return nil
}
