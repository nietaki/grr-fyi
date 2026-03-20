# Epstein File Review

## TODO

- [x] go back to using the standard echo FS (symlinks?)
- [x] move the endpoint to a path instead of /
- [x] cache only the right things
- [x] iframes
- [x] deescalate the privileges of the server container
- [x] HTML, AlpineJS
- [x] better header
- [ ] manually filter the ds9
- [x] separate deployment for development
- [x] separate the processing from the startup
- [x] process wav files
- [ ] serve converted wav files
- [x] unskip the ds9
- [x] sqlite
- [ ] sqlite shell
- [x] remove panics
- [x] handle (and learn) about context (cancellation)
- [x] JWT-like auth for the files

## sqlite db schema

- dataset
- path
- extension
- filetype
- filesize
- text_contents
- text_length
- salt (not really a salt, sin(random())

