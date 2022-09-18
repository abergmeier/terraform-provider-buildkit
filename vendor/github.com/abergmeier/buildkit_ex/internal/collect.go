package internal

import "github.com/moby/buildkit/frontend/dockerfile/instructions"

func collectSourcePaths(stages []instructions.Stage, sourcePaths chan<- string) {
	for _, s := range stages {
		collectSourcePathsOfStage(s, sourcePaths)
	}
}

func collectSourcePathsOfStage(s instructions.Stage, sourcePaths chan<- string) {

	for _, c := range s.Commands {
		switch t := c.(type) {
		case *instructions.AddCommand:
		case *instructions.CopyCommand:
			for _, sp := range t.SourcePaths {
				sourcePaths <- sp
			}
		}
	}
}
