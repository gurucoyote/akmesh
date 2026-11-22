package rgcmd

import (
	"git.site.quack-lab.dev/dave/gripgrep/filetype"
)

// buildTypes compiles -t/-T over the default type table. --type-add and
// --type-clear are deliberately absent (they are named as a deviation in the
// help text): they change the table rather than select from it, and an agent
// that wants them can pass a glob.
func buildTypes(cfg config) (*filetype.Matcher, error) {
	if len(cfg.types) == 0 && len(cfg.typesNot) == 0 {
		return nil, nil
	}
	b := filetype.NewBuilder()
	b.AddDefaults()
	changes := make([]filetype.Change, 0, len(cfg.types)+len(cfg.typesNot))
	for _, t := range cfg.types {
		changes = append(changes, filetype.Change{Kind: filetype.Select, Arg: t})
	}
	for _, t := range cfg.typesNot {
		changes = append(changes, filetype.Change{Kind: filetype.Negate, Arg: t})
	}
	if err := b.Apply(changes); err != nil {
		return nil, err
	}
	return b.Build()
}
