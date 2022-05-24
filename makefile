all: install signaling

install: 
	@./build.sh install
signaling:
	@./build.sh signaling