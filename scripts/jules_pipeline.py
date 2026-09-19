#!/usr/bin/env python3
"""
Jules & Antigravity Pipeline Orchestrator

Jules REST API と GitHub CLI を連携させ、提案の取得・承認/却下、
GitHub PR の自動ローカル検証および安全なマージを一元的に処理するスクリプト。
"""

import sys
import os
import json
import argparse
import subprocess
import urllib.request
import urllib.error
import ctypes
from ctypes import wintypes
from typing import Optional, Dict, Any, List

# Windows Credential API 定義
class CREDENTIAL(ctypes.Structure):
    _fields_ = [
        ('Flags', wintypes.DWORD),
        ('Type', wintypes.DWORD),
        ('TargetName', wintypes.LPWSTR),
        ('Comment', wintypes.LPWSTR),
        ('LastWritten', wintypes.FILETIME),
        ('CredentialBlobSize', wintypes.DWORD),
        ('CredentialBlob', ctypes.POINTER(ctypes.c_byte)),
        ('Persist', wintypes.DWORD),
        ('AttributeCount', wintypes.DWORD),
        ('Attributes', ctypes.c_void_p),
        ('TargetAlias', wintypes.LPWSTR),
        ('UserName', wintypes.LPWSTR),
    ]

def get_jules_token() -> str:
    """Windows Credential Manager から jules-cli:default のアクセストークンを取得"""
    advapi32 = ctypes.windll.advapi32
    CredReadW = advapi32.CredReadW
    CredReadW.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD, ctypes.POINTER(ctypes.POINTER(CREDENTIAL))]
    CredReadW.restype = wintypes.BOOL
    CredFree = advapi32.CredFree
    CredFree.argtypes = [ctypes.c_void_p]

    pcred = ctypes.POINTER(CREDENTIAL)()
    res = CredReadW('jules-cli:default', 1, 0, ctypes.byref(pcred))
    if not res:
        raise RuntimeError('Windows Credential Manager に jules-cli:default が見つかりません。jules auth を実行してください。')

    blob = ctypes.string_at(pcred.contents.CredentialBlob, pcred.contents.CredentialBlobSize)
    CredFree(pcred)
    token_data = json.loads(blob.decode('utf-8'))
    return token_data['access_token']

def api_request(endpoint: str, method: str = 'GET', data: Optional[Dict[str, Any]] = None) -> Any:
    """Jules 内部 API (swebot) へリクエストを送信"""
    token = get_jules_token()
    base_url = "https://aida.googleapis.com/v1/swebot"
    url = f"{base_url}/{endpoint.lstrip('/')}"
    
    headers = {
        'Authorization': f'Bearer {token}',
        'Content-Type': 'application/json'
    }
    
    payload = json.dumps(data).encode('utf-8') if data else None
    req = urllib.request.Request(url, data=payload, headers=headers, method=method)
    
    try:
        with urllib.request.urlopen(req) as resp:
            resp_body = resp.read().decode('utf-8')
            return json.loads(resp_body) if resp_body else {}
    except urllib.error.HTTPError as e:
        err_msg = e.read().decode('utf-8', errors='ignore')
        raise RuntimeError(f"API Error ({e.code}): {err_msg}")

def list_tasks(repo_filter: str = "gnb-memorymcp") -> List[Dict[str, Any]]:
    """リポジトリに関連するタスク/提案一覧を取得"""
    token = get_jules_token()
    base_url = "https://aida.googleapis.com/v1/swebot/tasks"
    page_token = None
    all_tasks = []

    while True:
        url = f"{base_url}?pageSize=50"
        if page_token:
            url += f"&pageToken={page_token}"
        req = urllib.request.Request(url, headers={'Authorization': f'Bearer {token}'})
        try:
            with urllib.request.urlopen(req) as resp:
                data = json.loads(resp.read().decode('utf-8'))
                tasks = data.get('tasks', [])
                for t in tasks:
                    full_dump = json.dumps(t).lower()
                    if repo_filter.lower() in full_dump:
                        all_tasks.append(t)
                page_token = data.get('nextPageToken')
                if not page_token or not tasks or len(all_tasks) >= 50:
                    break
        except Exception as e:
            print(f"Error fetching tasks: {e}", file=sys.stderr)
            break
    return all_tasks

def interact_task(task_id: str, message: str) -> Dict[str, Any]:
    """提案やタスクにメッセージを送信・指示"""
    endpoint = f"tasks/{task_id}:interact"
    payload = {
        "message": {
            "text": message
        }
    }
    return api_request(endpoint, method='POST', data=payload)

def delete_task(task_id: str) -> Any:
    """提案・タスクをアーカイブ（削除）"""
    endpoint = f"tasks/{task_id}"
    return api_request(endpoint, method='DELETE')

def run_cmd(cmd: List[str], cwd: Optional[str] = None) -> subprocess.CompletedProcess:
    """シェルコマンドを実行して標準出力とエラーを捕捉"""
    return subprocess.run(cmd, cwd=cwd, text=True, capture_output=True)

def list_prs(state: str = "open") -> List[Dict[str, Any]]:
    """GitHub CLI (gh) を用いて PR 一覧を取得"""
    res = run_cmd(["gh", "pr", "list", "--state", state, "--json", "number,title,headRefName,url,state"])
    if res.returncode != 0:
        raise RuntimeError(f"gh pr list failed: {res.stderr}")
    return json.loads(res.stdout)

def verify_pr(pr_number: int, test_command: str = "go test ./...") -> bool:
    """PR をローカルでチェックアウトし、テストを実行して検証"""
    print(f"\n==================================================")
    print(f"[*] Verifying PR #{pr_number}")
    print(f"==================================================")
    
    # 1. PR チェックアウト
    print(f"[1/3] Checking out PR #{pr_number}...")
    res = run_cmd(["gh", "pr", "checkout", str(pr_number)])
    if res.returncode != 0:
        print(f"[-] Failed to checkout PR #{pr_number}: {res.stderr}", file=sys.stderr)
        return False
    print(f"[+] Checkout successful.")

    # 2. 変更差分の表示 (要約)
    diff_res = run_cmd(["git", "diff", "--stat", "main...HEAD"])
    print(f"\n[2/3] Changed files:")
    print(diff_res.stdout.strip())

    # 3. テスト実行
    print(f"\n[3/3] Running verification test: `{test_command}`...")
    test_res = run_cmd(test_command.split())
    print(test_res.stdout)
    if test_res.stderr:
        print(test_res.stderr, file=sys.stderr)

    if test_res.returncode == 0:
        print(f"[SUCCESS] PR #{pr_number} passed all verification tests!")
        return True
    else:
        print(f"[FAILED] PR #{pr_number} failed tests with exit code {test_res.returncode}")
        return False

def merge_pr(pr_number: int, method: str = "squash", delete_branch: bool = True) -> bool:
    """PR を安全にマージ"""
    print(f"\n[*] Merging PR #{pr_number} via {method}...")
    cmd = ["gh", "pr", "merge", str(pr_number), f"--{method}"]
    if delete_branch:
        cmd.append("--delete-branch")
        
    res = run_cmd(cmd)
    if res.returncode == 0:
        print(f"[SUCCESS] PR #{pr_number} merged successfully!")
        return True
    else:
        print(f"[-] Merge failed: {res.stderr}", file=sys.stderr)
        return False

def main():
    parser = argparse.ArgumentParser(description="Jules & Antigravity Pipeline Orchestrator")
    subparsers = parser.add_subparsers(dest="command", required=True)

    # list-tasks
    list_t = subparsers.add_parser("list-tasks", help="List Jules proactive suggestions & tasks")
    list_t.add_argument("--repo", default="gnb-memorymcp", help="Repository filter")

    # interact
    interact_t = subparsers.add_parser("interact", help="Send response message to a task")
    interact_t.add_argument("task_id", help="Jules task ID")
    interact_t.add_argument("message", help="Message text")

    # delete-task
    delete_t = subparsers.add_parser("delete-task", help="Archive/delete a task")
    delete_t.add_argument("task_id", help="Jules task ID")

    # list-prs
    list_pr = subparsers.add_parser("list-prs", help="List GitHub Pull Requests")
    list_pr.add_argument("--state", default="open", choices=["open", "closed", "all"])

    # verify
    verify_p = subparsers.add_parser("verify", help="Checkout and verify PR locally")
    verify_p.add_argument("pr", type=int, help="PR number")
    verify_p.add_argument("--test-cmd", default="go test ./...", help="Test command to run")

    # merge
    merge_p = subparsers.add_parser("merge", help="Merge PR after verification")
    merge_p.add_argument("pr", type=int, help="PR number")
    merge_p.add_argument("--method", default="squash", choices=["squash", "merge", "rebase"])

    # auto-pipeline
    auto_p = subparsers.add_parser("auto-pipeline", help="Verify and merge PR in one shot")
    auto_p.add_argument("pr", type=int, help="PR number")
    auto_p.add_argument("--test-cmd", default="go test ./...", help="Test command")
    auto_p.add_argument("--method", default="squash", choices=["squash", "merge", "rebase"])

    args = parser.parse_args()

    if args.command == "list-tasks":
        tasks = list_tasks(args.repo)
        print(f"Found {len(tasks)} tasks for '{args.repo}':")
        for t in tasks:
            name = t.get("name", "")
            title = t.get("title", "No Title")
            state = t.get("state", "UNKNOWN")
            print(f"- [{state}] {name}: {title}")

    elif args.command == "interact":
        res = interact_task(args.task_id, args.message)
        print(f"Response: {res}")

    elif args.command == "delete-task":
        res = delete_task(args.task_id)
        print(f"Deleted task {args.task_id}: {res}")

    elif args.command == "list-prs":
        prs = list_prs(args.state)
        print(f"Found {len(prs)} PRs:")
        for pr in prs:
            print(f"#{pr['number']} [{pr['state']}] {pr['title']} ({pr['headRefName']})")

    elif args.command == "verify":
        success = verify_pr(args.pr, args.test_cmd)
        sys.exit(0 if success else 1)

    elif args.command == "merge":
        success = merge_pr(args.pr, args.method)
        sys.exit(0 if success else 1)

    elif args.command == "auto-pipeline":
        if verify_pr(args.pr, args.test_cmd):
            # Switch back to main to ensure merge can happen cleanly
            run_cmd(["git", "checkout", "main"])
            run_cmd(["git", "pull", "--rebase"])
            if merge_pr(args.pr, args.method):
                run_cmd(["git", "checkout", "main"])
                run_cmd(["git", "pull", "--rebase"])
                print(f"[SUCCESS] Pipeline complete for PR #{args.pr}")
                sys.exit(0)
        sys.exit(1)

if __name__ == '__main__':
    main()
