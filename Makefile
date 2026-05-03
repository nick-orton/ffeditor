PORTNAME=	ffeditor

GO?=		go
GOFLAGS?=

PREFIX?=	/usr/local
BINDIR?=	${PREFIX}/bin
DESTDIR?=

INSTALL?=		install
INSTALL_PROGRAM?=	${INSTALL} -s -m 0555
MKDIR?=			mkdir -p

BINARY=		${PORTNAME}

.PHONY: all build test test-short install clean

all: test build

build:
	${GO} build ${GOFLAGS} -o ${BINARY} .

test:
	${GO} test ${GOFLAGS} ./...

test-short:
	${GO} test -short ${GOFLAGS} ./...

install: test build
	${MKDIR} ${DESTDIR}${BINDIR}
	${INSTALL_PROGRAM} ${BINARY} ${DESTDIR}${BINDIR}/${BINARY}

clean:
	rm -f ${BINARY}
