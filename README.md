# spfmacro

## Name

*spfmacro* - Takes a various number of SPF records and serves an SPF macro

## Description

The example plugin prints "example" on every query that got handled by the server. It serves as
documentation for writing CoreDNS plugins.

## Compilation

This package will always be compiled as part of CoreDNS and not in a standalone way. It will require you to use `go get` or as a dependency on [plugin.cfg](https://github.com/coredns/coredns/blob/master/plugin.cfg).

The [manual](https://coredns.io/manual/toc/#what-is-coredns) will have more information about how to configure and extend the server with external plugins.

A simple way to consume this plugin, is by adding the following on [plugin.cfg](https://github.com/coredns/coredns/blob/master/plugin.cfg), and recompile it as [detailed on coredns.io](https://coredns.io/2017/07/25/compile-time-enabling-or-disabling-plugins/#build-with-compile-time-configuration-file).

~~~
spfmacro:github.com/slash2314/spfmacro
~~~


After this you can compile coredns by:

``` sh
go generate
go build
```

Or you can instead use make:

``` sh
make
```

## Syntax
### Examples
~~~ txt
spfmacro txt:_despf.mail.example.com
~~~
~~~ txt
spfmacro txt:amazonses.com txt:spf.protection.outlook.com
~~~
## Configuration
Your actual SPF record can look like this
~~~ txt
v=spf1 include:{i}.{d}._spf.example.com
~~~
where _spf.example.com is represented by the server that runs the coredns instance.

## Metrics

If monitoring is enabled (via the *prometheus* directive) the following metric is exported:

* `coredns_spfmacro_request_count_total{server}` - query count to the *spfmacro* plugin.

The `server` label indicated which server handled the request, see the *metrics* plugin for details.

## Ready

This plugin reports readiness to the ready plugin. It will be immediately ready.

## Examples

In this configuration, we set the domain that will send as to mail.example.com using SPF records from the listed services:
~~~ corefile
mail.example.com._spf.example.com:54 {
    log
    spfmacro txt:amazonses.com txt:spf.protection.outlook.com
}
~~~

This can be tested using the following dig:
~~~ txt
dig 199.255.192.1.mail.example.com._spf.example.com TXT @127.0.0.1 -p 54
~~~
and it will return
~~~ txt
;; ANSWER SECTION:
199.255.192.1.mail.example.com._spf.example.com. 30 IN TXT "v=spf1 -all"
~~~
## Also See
See the [manual](https://coredns.io/manual).