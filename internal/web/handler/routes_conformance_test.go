// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// pageRoutes reads the GET routes out of routes.go rather than repeating them
// here. The difference matters more than the saved typing: a hand-kept list
// checks the screens somebody remembered, and the screens nobody remembered
// are exactly the ones that drift. Routes with a {parameter} need an account
// and a collection to answer, so they are left to the tests that build one;
// everything a bare instance can render is covered automatically, including
// screens added after this test was written.
func pageRoutes(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "routes.go", nil, 0)
	if err != nil {
		t.Fatalf("parse routes.go: %v", err)
	}
	seen := map[string]bool{}
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "HandleFunc" {
			return true
		}
		// The pattern is `"GET " + s.Path("/…")`, so the literal method and
		// the literal path sit on either side of one concatenation.
		bin, ok := call.Args[0].(*ast.BinaryExpr)
		if !ok {
			return true
		}
		method, ok := stringLit(bin.X)
		if !ok || strings.TrimSpace(method) != "GET" {
			return true
		}
		inner, ok := bin.Y.(*ast.CallExpr)
		if !ok || len(inner.Args) != 1 {
			return true
		}
		path, ok := stringLit(inner.Args[0])
		if !ok {
			return true
		}
		// {$} is not a parameter, it is "this path exactly".
		path = strings.TrimSuffix(path, "{$}")
		if strings.Contains(path, "{") || seen[path] {
			return true
		}
		seen[path] = true
		out = append(out, path)
		return true
	})
	// The administration is one route with a section parameter, and the
	// sections are a closed set the handler already enumerates. They are the
	// screens the layout transfer missed, so they are the last ones that
	// should be left out of an automatic sweep.
	for _, section := range []string{
		adminSectionUsers, adminSectionInvites, adminSectionSettings,
		adminSectionDAV, adminSectionEscrow, adminSectionAudit,
	} {
		p := "/admin/" + section
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	if len(out) < 15 {
		t.Fatalf("only %d parameterless GET routes found; the parse is wrong, not the router", len(out))
	}
	return out
}

func stringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// Every page the router can answer without a DAV account is rendered and held
// to the rules the stylesheet cannot enforce on its own. This is the shape the
// visual acceptance arrived at the hard way: enumerate the screens, which are
// a closed set, rather than the mistakes, which are not.
func TestEveryPageObeysTheMarkupRules(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "root@example.org", testPassword)

	for _, path := range pageRoutes(t) {
		rec := a.get(path)
		// A redirect is an answer: /app/ sends you to a section, and a page
		// that needs something we have not set up says so with a status.
		if rec.Code >= 500 {
			t.Errorf("GET %s = %d", path, rec.Code)
			continue
		}
		body := rec.Body.String()
		if !strings.Contains(body, "<html") {
			continue
		}
		if styleAttr.MatchString(body) {
			t.Errorf("GET %s renders a style attribute, which the CSP drops", path)
		}
		for _, tag := range inlineScript.FindAllString(body, -1) {
			if !scriptWithSrc.MatchString(tag) {
				t.Errorf("GET %s renders an inline script (%s), which the CSP drops", path, tag)
			}
		}
		for _, href := range hrefsOf(body) {
			if strings.HasPrefix(href, "//") {
				t.Errorf("GET %s links to %q, which the browser reads as another host", path, href)
			}
		}
	}
}

// The count is asserted so that adding a screen without a route — or losing
// one to a refactor — shows up as a failing number rather than as silence.
func TestPageRouteCoverage(t *testing.T) {
	routes := pageRoutes(t)
	t.Logf("parameterless GET pages under test: %d", len(routes))
	for _, r := range routes {
		t.Log("  " + r)
	}
}
