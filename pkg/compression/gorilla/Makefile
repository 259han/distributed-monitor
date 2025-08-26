CXX=g++
CXXFLAGS=-std=c++11 -O3 -fPIC -Wall -Wextra
LDFLAGS=-shared

all: libgorilla.so

libgorilla.so: gorilla.o
	$(CXX) $(LDFLAGS) -o $@ $^

gorilla.o: gorilla.cpp gorilla.h
	$(CXX) $(CXXFLAGS) -c $< -o $@

clean:
	rm -f *.o *.so

.PHONY: all clean
