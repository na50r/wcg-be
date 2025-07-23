# WomboCombo Go  - Backend
This is the backend code for the WomboCombo Go game, a successor of the original [WomboCombo project](https://github.com/sopra-fs24-group-41) which was built by a group of students as part of the Sopra Spring 2024 course at the University of Zurich. The original WomboCombo was essentially a clone of the game InfiniteCraft by Neal Agrawal feauturing game modes, daily challenges and achievements. It used Java as the backend. WomboCombo Go uses Golang as the backend and implements certain parts of the original WomboCombo differently.

## Requirements
- Golang 1.23 (windows/amd64)
- SQLite3 (Requires [TDM-GCC](https://jmeubank.github.io/tdm-gcc/))
- Optional: Docker for Postgres

## Setup
```sh
make run #Builds and runs the server
make seed #Seeds the database with images, words and combinations
```

## API
The API is documented using Swagger. It can be accessed at `http://localhost:<port>/swagger/index.html` after running the server.