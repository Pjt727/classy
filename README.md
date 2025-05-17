# Overview
Classy is a **WIP** project made to provide a centralized API to access basic class information for many schools. 
It is aimed to be used by developers
who want to create apps for schools (think something like [coursicle](https://www.coursicle.com/)),
but do not want or cannot scrape the information themselves.

## how can you use classy?
The [classy-api](https://github.com/Pjt727/classy-api) can be viewed [here](https://pjt727.github.io/classy-api/) (only works if you are locally running the classy server).
Class information can also be synced with existing data sources to provide updates instead of complete class data, [class-sync](https://github.com/Pjt727/classy-sync)
is a tool to sync to a database (currently only sqlite).

# Technical Notes
- any .http files follow syntax of [kuala.nvim](https://github.com/mistweaverco/kulala.nvim) a http client as a neovim plugin
    - these files are not accessed by the code and are there for exploratory purposes allowing developers to reproduce the minimal requests needed to get class information
- Database: Postgres
- Main Language: Golang
    - logging: [logrus](https://github.com/sirupsen/logrus)
    - database interaction: [sqlc](https://docs.sqlc.dev/en/latest/) with [pgx](https://github.com/jackc/pgx)
    - cmd: [cobra](https://github.com/spf13/cobra)

