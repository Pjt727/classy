# Overview
Classy is a WIP project made to provide a centralized API to access basic class information for many schools. 
It is aimed to be used by developers
who want to create apps for schools (think something like [coursicle](https://www.coursicle.com/)),
but do not want or cannot scrape the information themselves.

# Techical Notes
- **DISCLAIMER**: this project is very new: tech stack and code organization is subject to change
    - consistancy and naming conventions within the codebase is evolving and thus may be inconsistent
- any .http files follow syntax of [kuala.nvim](https://github.com/mistweaverco/kulala.nvim) a http client as a neovim plugin
    - these files are not accessed by the code and are there for exploratory purposes allowing developers to reproduce the minimal requests needed to get class information
- Database: Postgres
- Main Language: golang
    - logging: [logrus](https://github.com/sirupsen/logrus)
    - database interaction: [sqlc](https://docs.sqlc.dev/en/latest/) with [pgx](https://github.com/jackc/pgx)
    - cmd: [cobra](https://github.com/spf13/cobra)

