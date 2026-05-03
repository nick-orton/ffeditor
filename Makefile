PORTNAME=	ffeditor

GO?=		go
GOFLAGS?=

BINARY=		${PORTNAME}

.PHONY: all build test test-short clean

all: test build

build:
	${GO} build ${GOFLAGS} -o ${BINARY} .

test:
	${GO} test ${GOFLAGS} ./...

test-short:
	${GO} test -short ${GOFLAGS} ./...

clean:
	rm -f ${BINARY}
