go-serversets/fixedset [![Build Status](https://travis-ci.org/thinker0/go-serversets.png?branch=master)](https://travis-ci.org/thinker0/go-serversets) [![Godoc Reference](https://godoc.org/github.com/thinker0/go-serversets?status.png)](https://godoc.org/github.com/thinker0/go-serversets/fixedset)
=====================

Fixed set, i.e. severset without the zookeeper. This package implements a stub for `Watch`
where endpoints are set manually vs. via zookeeper. This is useful for testing or
to take advantage of the load balancing packages without the discovery part.
