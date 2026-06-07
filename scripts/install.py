#!/usr/bin/env python3
import os
import sys
import shutil
import subprocess
import argparse

def get_repo_root():
    # Since install.py is in scripts/, repo root is one level up
    script_dir = os.path.dirname(os.path.abspath(__file__))
    return os.path.dirname(script_dir)

def check_command(cmd):
    return shutil.which(cmd) is not None

def setup_config_env(repo_root):
    config_dir = os.path.expanduser("~/.grabby")
    os.makedirs(config_dir, exist_ok=True)
    env_file = os.path.join(config_dir, ".env")
    
    # 1. Copy env.example if .env doesn't exist
    if not os.path.exists(env_file):
        example_src = os.path.join(repo_root, "python-server", ".env.example")
        if not os.path.exists(example_src):
            example_src = os.path.join(repo_root, "go-server", ".env.example")
            
        if os.path.exists(example_src):
            print(f"📝 正在创建配置文件: {env_file}")
            shutil.copy(example_src, env_file)
        else:
            print(f"📝 正在创建空白配置文件: {env_file}")
            with open(env_file, "w", encoding="utf-8") as f:
                f.write("PORT=5040\n")
                
    # 2. Add or update GRABBY_PROJECT_DIR in .env
    lines = []
    project_dir_exists = False
    if os.path.exists(env_file):
        with open(env_file, "r", encoding="utf-8") as f:
            lines = f.readlines()
            
    new_lines = []
    for line in lines:
        if line.strip().startswith("GRABBY_PROJECT_DIR="):
            new_lines.append(f"GRABBY_PROJECT_DIR={repo_root}\n")
            project_dir_exists = True
        else:
            new_lines.append(line)
            
    if not project_dir_exists:
        if new_lines and not new_lines[-1].endswith("\n"):
            new_lines.append("\n")
        new_lines.append(f"GRABBY_PROJECT_DIR={repo_root}\n")
        
    with open(env_file, "w", encoding="utf-8") as f:
        f.writelines(new_lines)
        
    print(f"✅ 配置文件初始化完成 (已绑定项目根目录: {repo_root})")
    return config_dir

def install_python_cli(repo_root, install_dir):
    cli_src = os.path.join(repo_root, "python-cli", "cli.py")
    if not os.path.exists(cli_src):
        print(f"❌ 错误: 找不到 Python CLI 源码文件: {cli_src}", file=sys.stderr)
        sys.exit(1)
        
    os.makedirs(install_dir, exist_ok=True)
    dst = os.path.join(install_dir, "grabby")
    
    print(f"📦 正在安装 Python CLI 到 {dst}...")
    shutil.copy(cli_src, dst)
    
    # Make executable on POSIX systems
    if os.name == 'posix':
        try:
            os.chmod(dst, 0o755)
        except Exception as e:
            print(f"⚠️  警告: 无法设置可执行权限: {e}", file=sys.stderr)
            
    print("✅ Python CLI 安装成功！")

def install_go_cli(repo_root, install_dir):
    go_cli_dir = os.path.join(repo_root, "go-cli")
    if not os.path.exists(filepath := os.path.join(go_cli_dir, "main.go")):
        print(f"❌ 错误: 找不到 Go CLI 源码文件: {filepath}", file=sys.stderr)
        sys.exit(1)
        
    if not check_command("go"):
        print("❌ 错误: 未检测到 Go 环境！", file=sys.stderr)
        print("   安装 Go CLI 需要本地 Go 编译器。请先安装 Go: https://go.dev/doc/install", file=sys.stderr)
        print("   或者选择安装 Python CLI 版本 (无需 Go 环境)。", file=sys.stderr)
        sys.exit(1)
        
    print("📡 正在本地编译 Go CLI 二进制文件...")
    try:
        # Build binary locally (to match architecture and avoid gatekeeper issues)
        subprocess.run(
            ["go", "build", "-o", "grabby", "main.go"],
            cwd=go_cli_dir,
            check=True
        )
    except subprocess.CalledProcessError as e:
        print(f"❌ 编译 Go CLI 失败: {e}", file=sys.stderr)
        sys.exit(1)
        
    os.makedirs(install_dir, exist_ok=True)
    src_binary = os.path.join(go_cli_dir, "grabby")
    dst_binary = os.path.join(install_dir, "grabby")
    
    print(f"📦 正在拷贝编译后的二进制到 {dst_binary}...")
    shutil.copy(src_binary, dst_binary)
    
    # Clean up local build product inside src
    try:
        os.remove(src_binary)
    except Exception:
        pass
        
    if os.name == 'posix':
        try:
            os.chmod(dst_binary, 0o755)
        except Exception:
            pass
            
    print("✅ Go CLI 编译安装成功！")

def check_path(install_dir):
    paths = os.environ.get("PATH", "").split(os.pathsep)
    norm_install_dir = os.path.abspath(install_dir)
    norm_paths = [os.path.abspath(p) for p in paths if p]
    
    if norm_install_dir not in norm_paths:
        print("\n" + "="*60)
        print("⚠️  警告: 安装目录不在系统的 PATH 路径中！")
        print(f"   安装目录: {install_dir}")
        print("   为了能在任何终端直接使用 grabby 命令，请将该路径添加到您的 shell 配置文件。")
        print("\n   对于 macOS / Linux 上的 zsh 用户，可运行以下命令：")
        print(f'   echo \'export PATH="$PATH:{install_dir}"\' >> ~/.zshrc')
        print("   然后运行: source ~/.zshrc")
        print("="*60 + "\n")
    else:
        print(f"\n🎉 恭喜！安装路径 {install_dir} 已包含在 PATH 中，您可以直接在终端运行 `grabby`！\n")

def main():
    repo_root = get_repo_root()
    default_install_dir = os.environ.get("GRABBY_INSTALL_DIR") or os.path.expanduser("~/.local/bin")
    
    parser = argparse.ArgumentParser(description="Grabby CLI 安装工具 (本地编译/脚本复制)")
    parser.add_argument("--type", choices=["python", "go"], help="指定安装 CLI 的类型 (python 或 go)")
    parser.add_argument("--install-dir", default=default_install_dir, help="指定可执行文件安装目录 (默认: ~/.local/bin)")
    
    args = parser.parse_args()
    
    print("🚀 开始安装 Grabby (网页内容抓取工具)...\n")
    
    install_type = args.type
    if not install_type:
        print("请选择要安装的 CLI 类型：")
        print("  1. Python CLI (推荐，免编译，适合 macOS，不会被 Gatekeeper 拦截)")
        print("  2. Go CLI (在本地基于源码编译，生成 native 二进制，需要安装 Go 编译器)")
        
        while True:
            choice = input("\n请输入数字 (1 或 2): ").strip()
            if choice == "1":
                install_type = "python"
                break
            elif choice == "2":
                install_type = "go"
                break
            else:
                print("无效输入，请输入 1 或 2。")
                
    if install_type == "python":
        install_python_cli(repo_root, args.install_dir)
    elif install_type == "go":
        install_go_cli(repo_root, args.install_dir)
        
    setup_config_env(repo_root)
    check_path(args.install_dir)
    
    print("🏁 安装完成！你可以使用以下命令开始使用：")
    print(f"   1. 启动服务: grabby start python  (或 grabby start go)")
    print("   2. 抓取页面: grabby extract https://example.com")

if __name__ == "__main__":
    main()
