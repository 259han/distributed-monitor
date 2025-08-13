#!/bin/bash

# 验证所有测试文件的脚本（最终版）

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔍 开始验证所有测试文件...${NC}"
echo "=================================="

# 计数器
TOTAL_FILES=0
VALID_FILES=0
INVALID_FILES=0

echo -e "${BLUE}📁 验证Go测试文件...${NC}"
if [[ -f "test/quic_test.go" ]]; then
    if go test ./test/ -v -timeout 10s 2>&1 | grep -q "PASS"; then
        echo -e "${GREEN}✅ quic_test.go 测试通过${NC}"
        ((VALID_FILES++))
    else
        echo -e "${RED}❌ quic_test.go 测试失败${NC}"
        ((INVALID_FILES++))
    fi
    ((TOTAL_FILES++))
fi

echo -e "\n${BLUE}📁 验证Shell测试脚本...${NC}"
for script in test/*.sh; do
    if [[ -f "$script" ]]; then
        if bash -n "$script" 2>/dev/null; then
            echo -e "${GREEN}✅ $(basename "$script") 语法正确${NC}"
            if [[ ! -x "$script" ]]; then
                chmod +x "$script"
                echo -e "  ${YELLOW}⚠️  已添加执行权限${NC}"
            fi
            ((VALID_FILES++))
        else
            echo -e "${RED}❌ $(basename "$script") 语法错误${NC}"
            ((INVALID_FILES++))
        fi
        ((TOTAL_FILES++))
    fi
done

echo -e "\n${BLUE}📁 验证配置文件...${NC}"
for config in configs/*.yaml; do
    if [[ -f "$config" ]]; then
        echo -e "${GREEN}✅ $(basename "$config") 存在${NC}"
        ((VALID_FILES++))
    fi
    ((TOTAL_FILES++))
done

echo -e "\n${BLUE}�� 验证QUIC协议实现...${NC}"
if [[ -d "pkg/quic" ]]; then
    echo -e "${GREEN}✅ QUIC包存在${NC}"
    if go build -o /dev/null ./pkg/quic/ 2>/dev/null; then
        echo -e "${GREEN}✅ QUIC包编译成功${NC}"
        ((VALID_FILES++))
    else
        echo -e "${RED}❌ QUIC包编译失败${NC}"
        ((INVALID_FILES++))
    fi
    ((TOTAL_FILES++))
else
    echo -e "${RED}❌ QUIC包不存在${NC}"
    ((INVALID_FILES++))
    ((TOTAL_FILES++))
fi

echo -e "\n${BLUE}📁 验证组件编译...${NC}"
for component in agent broker visualization; do
    if go build -buildvcs=false -o /dev/null ./$component/cmd 2>/dev/null; then
        echo -e "${GREEN}✅ $component 编译成功${NC}"
        ((VALID_FILES++))
    else
        echo -e "${RED}❌ $component 编译失败${NC}"
        ((INVALID_FILES++))
    fi
    ((TOTAL_FILES++))
done

echo -e "\n${BLUE}📊 验证结果统计...${NC}"
echo "=================================="
echo -e "总文件数: ${TOTAL_FILES}"
echo -e "有效文件: ${GREEN}${VALID_FILES}${NC}"
echo -e "无效文件: ${RED}${INVALID_FILES}${NC}"

if [[ $INVALID_FILES -eq 0 ]]; then
    echo -e "\n${GREEN}🎉 所有测试文件验证通过！${NC}"
    exit 0
else
    echo -e "\n${RED}❌ 发现 ${INVALID_FILES} 个问题，请修复后重新验证。${NC}"
    exit 1
fi
