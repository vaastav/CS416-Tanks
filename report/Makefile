report.pdf: REPORT.md
	pandoc --pdf-engine=pdflatex -o $@ $<

clean:
	$(RM) report.pdf

.PHONY: clean
