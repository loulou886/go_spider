// Copyright 2013 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

// This program generates internet protocol constants and tables by
// reading IANA protocol registries.
//
// Usage of this program:
//	go run gen.go > iana.go
package main

import (
    "bytes"
    "encoding/xml"
    "fmt"
    "go/format"
    "io"
    "net/http"
    "os"
    "strconv"
    "strings"
)

var registries = []struct {
    url   string
    parse func(io.Writer, io.Reader) error
}{
    {
        "http://www.iana.org/assignments/icmp-parameters/icmp-parameters.xml",
        parseICMPv4Parameters,
    },
    {
        "http://www.iana.org/assignments/protocol-numbers/protocol-numbers.xml",
        parseProtocolNumbers,
    },
}

func main() {
    var bb bytes.Buffer
    fmt.Fprintf(&bb, "// go run gen.go\n")
    fmt.Fprintf(&bb, "// GENERATED BY THE COMMAND ABOVE; DO NOT EDIT\n\n")
    fmt.Fprintf(&bb, "package ipv4\n\n")
    for _, r := range registries {
        resp, err := http.Get(r.url)
        if err != nil {
            fmt.Fprintln(os.Stderr, err)
            os.Exit(1)
        }
        defer resp.Body.Close()
        if resp.StatusCode != http.StatusOK {
            fmt.Fprintf(os.Stderr, "got HTTP status code %v for %v\n", resp.StatusCode, r.url)
            os.Exit(1)
        }
        if err := r.parse(&bb, resp.Body); err != nil {
            fmt.Fprintln(os.Stderr, err)
            os.Exit(1)
        }
        fmt.Fprintf(&bb, "\n")
    }
    b, err := format.Source(bb.Bytes())
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
    os.Stdout.Write(b)
}

func parseICMPv4Parameters(w io.Writer, r io.Reader) error {
    dec := xml.NewDecoder(r)
    var icp icmpv4Parameters
    if err := dec.Decode(&icp); err != nil {
        return err
    }
    prs := icp.escape()
    fmt.Fprintf(w, "// %s, Updated: %s\n", icp.Title, icp.Updated)
    fmt.Fprintf(w, "const (\n")
    for _, pr := range prs {
        if pr.Descr == "" {
            continue
        }
        fmt.Fprintf(w, "ICMPType%s ICMPType = %d", pr.Descr, pr.Value)
        fmt.Fprintf(w, "// %s\n", pr.OrigDescr)
    }
    fmt.Fprintf(w, ")\n\n")
    fmt.Fprintf(w, "// %s, Updated: %s\n", icp.Title, icp.Updated)
    fmt.Fprintf(w, "var icmpTypes = map[ICMPType]string{\n")
    for _, pr := range prs {
        if pr.Descr == "" {
            continue
        }
        fmt.Fprintf(w, "%d: %q,\n", pr.Value, strings.ToLower(pr.OrigDescr))
    }
    fmt.Fprintf(w, "}\n")
    return nil
}

type icmpv4Parameters struct {
    XMLName    xml.Name `xml:"registry"`
    Title      string   `xml:"title"`
    Updated    string   `xml:"updated"`
    Registries []struct {
        Title   string `xml:"title"`
        Records []struct {
            Value string `xml:"value"`
            Descr string `xml:"description"`
        }   `xml:"record"`
    }   `xml:"registry"`
}

type canonICMPv4ParamRecord struct {
    OrigDescr string
    Descr     string
    Value     int
}

func (icp *icmpv4Parameters) escape() []canonICMPv4ParamRecord {
    id := -1
    for i, r := range icp.Registries {
        if strings.Contains(r.Title, "Type") || strings.Contains(r.Title, "type") {
            id = i
            break
        }
    }
    if id < 0 {
        return nil
    }
    prs := make([]canonICMPv4ParamRecord, len(icp.Registries[id].Records))
    sr := strings.NewReplacer(
        "Messages", "",
        "Message", "",
        "ICMP", "",
        "+", "P",
        "-", "",
        "/", "",
        ".", "",
        " ", "",
    )
    for i, pr := range icp.Registries[id].Records {
        if strings.Contains(pr.Descr, "Reserved") ||
            strings.Contains(pr.Descr, "Unassigned") ||
            strings.Contains(pr.Descr, "Deprecated") ||
            strings.Contains(pr.Descr, "Experiment") ||
            strings.Contains(pr.Descr, "experiment") {
            continue
        }
        ss := strings.Split(pr.Descr, "\n")
        if len(ss) > 1 {
            prs[i].Descr = strings.Join(ss, " ")
        } else {
            prs[i].Descr = ss[0]
        }
        s := strings.TrimSpace(prs[i].Descr)
        prs[i].OrigDescr = s
        prs[i].Descr = sr.Replace(s)
        prs[i].Value, _ = strconv.Atoi(pr.Value)
    }
    return prs
}

func parseProtocolNumbers(w io.Writer, r io.Reader) error {
    dec := xml.NewDecoder(r)
    var pn protocolNumbers
    if err := dec.Decode(&pn); err != nil {
        return err
    }
    prs := pn.escape()
    prs = append([]canonProtocolRecord{{
        Name:  "IP",
        Descr: "IPv4 encapsulation, pseudo protocol number",
        Value: 0,
    }}, prs...)
    fmt.Fprintf(w, "// %s, Updated: %s\n", pn.Title, pn.Updated)
    fmt.Fprintf(w, "const (\n")
    for _, pr := range prs {
        if pr.Name == "" {
            continue
        }
        fmt.Fprintf(w, "ianaProtocol%s = %d", pr.Name, pr.Value)
        s := pr.Descr
        if s == "" {
            s = pr.OrigName
        }
        fmt.Fprintf(w, "// %s\n", s)
    }
    fmt.Fprintf(w, ")\n")
    return nil
}

type protocolNumbers struct {
    XMLName  xml.Name `xml:"registry"`
    Title    string   `xml:"title"`
    Updated  string   `xml:"updated"`
    RegTitle string   `xml:"registry>title"`
    Note     string   `xml:"registry>note"`
    Records  []struct {
        Value string `xml:"value"`
        Name  string `xml:"name"`
        Descr string `xml:"description"`
    }   `xml:"registry>record"`
}

type canonProtocolRecord struct {
    OrigName string
    Name     string
    Descr    string
    Value    int
}

func (pn *protocolNumbers) escape() []canonProtocolRecord {
    prs := make([]canonProtocolRecord, len(pn.Records))
    sr := strings.NewReplacer(
        "-in-", "in",
        "-within-", "within",
        "-over-", "over",
        "+", "P",
        "-", "",
        "/", "",
        ".", "",
        " ", "",
    )
    for i, pr := range pn.Records {
        prs[i].OrigName = pr.Name
        s := strings.TrimSpace(pr.Name)
        switch pr.Name {
        case "ISIS over IPv4":
            prs[i].Name = "ISIS"
        case "manet":
            prs[i].Name = "MANET"
        default:
            prs[i].Name = sr.Replace(s)
        }
        ss := strings.Split(pr.Descr, "\n")
        for i := range ss {
            ss[i] = strings.TrimSpace(ss[i])
        }
        if len(ss) > 1 {
            prs[i].Descr = strings.Join(ss, " ")
        } else {
            prs[i].Descr = ss[0]
        }
        prs[i].Value, _ = strconv.Atoi(pr.Value)
    }
    return prs
}