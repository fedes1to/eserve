# eserve
Make having a build server actually fun

## WARNING:
project needs a LOT of work, it's not even close to even halfway done,
it'll take a while, this is just a prototype, and it will only target
Gentoo for now, it is NOT WORKING as of now

## How it works
1. client checks if its in binpkgs
2. if not then it requests the server to build it
3. else it just pulls what exists

idea is to have it sync USE flags and also allow to have similar systems easily
(reproducibility)
