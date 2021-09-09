module github.com/mengelbart/rtq-go-endpoint

go 1.16

require (
	github.com/lucas-clemente/quic-go v0.22.1
	github.com/mengelbart/rtq-go v0.1.1-0.20210809141046-4810b3fdd52a
	github.com/mengelbart/scream-go v0.2.2
	github.com/pion/interceptor v0.0.13-0.20210819152811-ea60e4df22b3
	github.com/pion/rtcp v1.2.6
	github.com/pion/rtp v1.6.2
)

replace github.com/lucas-clemente/quic-go v0.22.1 => github.com/mengelbart/quic-go v0.7.1-0.20210909103716-3e25d1c2cf91
