test: clean
	for i in example_*.proto; do \
		echo "processing $$i..." && \
		protoc --twirp_ts_out=./ $$i; \
	done

clean:
	rm -f *.ts

lint:
	for i in example_*.ts; do \
		echo "processing $$i..." && \
		tslint $$i; \
	done
