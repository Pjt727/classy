# Overview
Classy is a **WIP** project made to provide a centralized API to access basic class information for many schools.
It is aimed to be help developers
create apps for colleges and universities (like [coursicle](https://www.coursicle.com/)).

## How can I use Classy?
Classy will eventually expose a public API to enable applications to get and sync class information.
### [classy-api](https://github.com/Pjt727/classy-api)
An explorer page for the basic API information.
It is hosted [here](https://pjt727.github.io/classy-api/) on GitHub, but will only work once Classy itself is hosted.
For now, it can only be used when running Classy locally as well.
### [classy-sync](https://github.com/Pjt727/classy-sync)
An application meant to help replicate all or some of Classy's database with other data stores.
These data stores can be other servers displaying content for many different schools, or local data stores on a client for fast lookups for a school.


The [classy-api](https://github.com/Pjt727/classy-api) can be viewed [here](https://pjt727.github.io/classy-api/) (only works if you are locally running the Classy server).
Class information can also be synced with existing data sources to provide updates instead of complete class data. [class-sync](https://github.com/Pjt727/classy-sync)
is a tool to sync to a database (currently only SQLite).

# Technical Notes
- Database: Postgres
- Main Language: Go
    - Logging: [logrus](https://github.com/sirupsen/logrus)
    - Database interaction: [sqlc](https://docs.sqlc.dev/en/latest/) with [pgx](https://github.com/jackc/pgx)
    - cmd: [cobra](https://github.com/spf13/cobra)
- Any .http files follow the syntax of [kuala.nvim](https://github.com/mistweaverco/kulala.nvim), an HTTP client for Neovim.
    - These files are not accessed by the code and are there for exploratory purposes, allowing developers to reproduce the minimal requests needed to get class information.
