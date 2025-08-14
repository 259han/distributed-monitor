# Makefile for Streaming Top-K C++ Library

CXX = g++
CXXFLAGS = -std=c++17 -O3 -Wall -Wextra -fPIC
INCLUDES = -I.
LIBS = -lpthread

# 目标库文件
LIB_NAME = libstreaming_topk.a
SHARED_LIB = libstreaming_topk.so

# 源文件
SOURCES = streaming_topk.cpp streaming_topk_c_wrapper.cpp
OBJECTS = $(SOURCES:.cpp=.o)

# 默认目标
all: $(LIB_NAME) $(SHARED_LIB)

# 静态库
$(LIB_NAME): $(OBJECTS)
	ar rcs $@ $(OBJECTS)
	@echo "Built static library: $@"

# 动态库
$(SHARED_LIB): $(OBJECTS)
	$(CXX) -shared -o $@ $(OBJECTS) $(LIBS)
	@echo "Built shared library: $@"

# 编译目标文件
%.o: %.cpp
	$(CXX) $(CXXFLAGS) $(INCLUDES) -c $< -o $@

# 测试程序
test: test_streaming_topk
	./test_streaming_topk

test_streaming_topk: test_streaming_topk.cpp $(LIB_NAME)
	$(CXX) $(CXXFLAGS) $(INCLUDES) -o $@ test_streaming_topk.cpp $(LIB_NAME) $(LIBS)

# 清理
clean:
	rm -f $(OBJECTS) $(LIB_NAME) $(SHARED_LIB) test_streaming_topk
	@echo "Cleaned build files"

# 安装到系统目录
install: $(SHARED_LIB)
	sudo cp $(SHARED_LIB) /usr/local/lib/
	sudo cp streaming_topk.h /usr/local/include/
	sudo cp streaming_topk_c_wrapper.h /usr/local/include/
	sudo ldconfig
	@echo "Installed to system directories"

.PHONY: all clean test install
