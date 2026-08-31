# uq-profile

UQ splits a course profile across tabs, so printing one gets you whichever tab
you happened to have open. This opens the whole thing as a single page, ready
for Ctrl+P.

```
uq-profile CSSE1001 2025 2      # opens it in your browser
uq-profile CSSE1001             # lists what's available
uq-profile -url CSSE1001 2025 2 # just the URL
```

Grab a binary from [releases](https://github.com/mmogr/uq-profile/releases), or
`go install github.com/mmogr/uq-profile@latest`.

It fetches the course page to find that offering's profile, then unhides the
sections UQ tabs away (they're already in the page, marked `class="hidden"`).
Two requests, only when you ask. Profiles older than about 2024 live on UQ's
archive, which has a different layout — those just open as-is.
