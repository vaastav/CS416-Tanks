proposal.pdf: PROPOSAL.md
	pandoc --pdf-engine=pdflatex -o $@ $<

clean:
	$(RM) proposal.pdf

.PHONY: clean
