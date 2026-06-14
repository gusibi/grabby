import * as React from "react";
import { Loader2, KeyRound } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { api } from "@/lib/api";

interface AuthDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onAuthenticated: () => void;
}

export function AuthDialog({ open, onOpenChange, onAuthenticated }: AuthDialogProps) {
  const [key, setKey] = React.useState("");
  const [error, setError] = React.useState("");
  const [isSubmitting, setIsSubmitting] = React.useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setIsSubmitting(true);
    try {
      const data = await api.login(key);
      if (data.success && data.authenticated) {
        setKey("");
        onAuthenticated();
        onOpenChange(false);
      } else {
        setError(data.error || "密钥不正确");
      }
    } catch (err) {
      console.error("Login failed", err);
      setError("登录失败，请检查服务状态");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <KeyRound className="w-4 h-4 text-indigo-500" />
              管理授权
            </DialogTitle>
            <DialogDescription>输入管理员密钥后可使用设置和生成控制。</DialogDescription>
          </DialogHeader>

          <div className="flex flex-col gap-2">
            <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">
              管理密钥
            </label>
            <Input
              autoFocus
              type="password"
              value={key}
              onChange={(e) => setKey(e.target.value)}
              placeholder="输入密钥"
              className="h-9 text-sm"
            />
            {error && <p className="text-xs font-semibold text-rose-500">{error}</p>}
          </div>

          <DialogFooter>
            <Button type="submit" disabled={isSubmitting || !key.trim()} className="h-9 text-xs">
              {isSubmitting ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : null}
              登录
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
