CXX=g++
CXXFLAGS=-std=c++11 -O3 -march=native -fPIC -Wall -Wextra
LDFLAGS=-shared

all: libsimd.so

libsimd.so: simd.o
	$(CXX) $(LDFLAGS) -o $@ $^

simd.o: simd.cpp simd.h
	$(CXX) $(CXXFLAGS) -c $< -o $@

clean:
	rm -f *.o *.so

.PHONY: all clean
