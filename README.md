# Overview
Classy is a **WIP** project made to provide a centralized API to access basic class information for many schools.
It is aimed to be help developers
create apps for colleges and universities like [coursicle](https://www.coursicle.com/).

## How can I use Classy?
Classy will eventually expose a public API to enable applications to get and sync class information.
### [classy-api](https://github.com/Pjt727/classy-api)
An explorer page for the basic API information.
It is hosted [here](https://pjt727.github.io/classy-api/) on GitHub, but will only work once Classy itself is hosted.
For now, it can only be used when running Classy locally.
### [classy-sync](https://github.com/Pjt727/classy-sync)
Help replicate all or some of Classy's database with other data stores.


# Technical Notes
- Database: Postgres
- Main Language: Go
    - Logging: slog
    - Database interaction: [sqlc](https://docs.sqlc.dev/en/latest/) with [pgx](https://github.com/jackc/pgx)
    - cmd: [cobra](https://github.com/spf13/cobra)
- Any .http files follow the syntax of [kuala.nvim](https://github.com/mistweaverco/kulala.nvim), an HTTP client for Neovim.
    - These files are not accessed by the code and are there for exploratory purposes, allowing developers to reproduce the minimal requests needed to get class information.
