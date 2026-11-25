# Add GitHub issue forms

The server repository offers forms for bug reports, feature requests, and documentation
issues. Bug reports ask for the server version and reproduction steps, with space for relevant
setup information and redacted diagnostics. Feature requests ask about the user's goal and the
desired behaviour. Each form applies an existing repository label.

The issue chooser links to setup help and the app repositories. Blank issues remain available
for reports that do not fit the forms.

Validation covers YAML parsing, GitHub form structure, labels, and link destinations. GitHub
displays the forms in the issue chooser once they are on the default branch. This change does
not affect the Arc runtime or logging.
