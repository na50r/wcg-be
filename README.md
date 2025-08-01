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

## Docker
```
make docker-build
API_KEY=<COHERE_API_KEY> make docker-seed
```

## API
The API is documented using Swagger. It can be accessed at `http://localhost:<port>/swagger/index.html` after running the server.

## Seeding Data
The database is seeded with some initial data, namely:
* Icons for profile pictures
* Combinations based on on Infinite Craft

The icons were taken from @wayou's [anonymous-animals](https://github.com/wayou/anonymous-animals)

The combinations are from @napstaa967's [infinite-craft-database](https://github.com/napstaa967/infinite-craft-database/blob/main/items.json), JSON was converted and filtered to the appropriate CSV using a Python script. The CSV files can be found [here](https://drive.google.com/drive/folders/18pcu6pGdO9eN8S_FBiOg6PQXcBCe52YO?usp=drive_link)

