# Game Analysis Tool

## Overview

Game Analysis Tool is a Go application that calculates and tracks player rankings using the Elo rating system. The tool is designed to analyze game results (particularly Magic: The Gathering games) from CSV data and provide statistical insights about player performance over time.

## Features

- **Elo Rating Calculation**: Implements the Elo rating system to provide an objective measure of player skill
- **CSV Data Processing**: Reads game records from a CSV file to calculate ratings
- **Player Performance Tracking**: Maintains and updates player ratings based on game outcomes
- **Flexible Input**: Supports a variety of game formats where players are ranked from winner to losers
- **Terminal User Interface**: View rankings in an interactive TUI with navigation controls

## Requirements

- Go 1.17 or higher
- Dependencies:
  - github.com/kortemy/elo-go

## Installation

```bash
# Clone the repository
git clone https://github.com/dylanlott/game-analysis.git
cd game-analysis

# Build the application
go build
```

## Usage

1. Prepare your game data in a CSV file with the following format:
   - First column: Empty (or can contain identifier)
   - Second column: Date of the game
   - Subsequent columns: Players in order of finish (winner first)

2. Run the application:

```bash
# Using the default CSV file (mtgscores.csv)
./game-analysis

# Using a custom CSV file
./game-analysis -path=path/to/your/data.csv

# Display rankings with Terminal User Interface
./game-analysis -tui
```

### Terminal User Interface

The `-tui` flag enables an interactive Terminal User Interface for viewing player rankings:

- Navigate through the rankings using the **up** and **down** arrow keys
- Exit the TUI by pressing **q** or **Ctrl+C**

This provides a more interactive way to browse player rankings, especially when dealing with large player pools.

## Data Format Example

The expected CSV format looks like:

```csv
,2022-01-01,PlayerA,PlayerB,PlayerC
,2022-01-08,PlayerB,PlayerA,PlayerD
```

Where:

- The first column is empty
- The second column contains the date
- The remaining columns list players in finishing order (winner to losers)

## How It Works

The application:

1. Reads the game data from the specified CSV file
2. Initializes each player with a base Elo rating
3. For each game, updates player ratings based on their performance
4. Calculates and displays the final Elo ratings for all players

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
