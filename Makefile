proposal.pdf: PROPOSAL.md
	pandoc --pdf-engine=xelatex -o $@ $<

clean:
	$(RM) proposal.pdf

.PHONY: clean