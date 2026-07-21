import * as React from "react";
import { KeyRound } from "lucide-react";
import { Dialog, DialogHeader } from "@astryxdesign/core/Dialog";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Button } from "@astryxdesign/core/Button";
import { VStack, HStack } from "@astryxdesign/core/Layout";
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
    <Dialog 
      isOpen={open} 
      onOpenChange={onOpenChange}
      purpose="form"
      width={400}
    >
      <form onSubmit={handleSubmit}>
        <DialogHeader
          title="管理授权"
          subtitle="输入管理员密钥后可使用设置和生成控制。"
          startContent={<KeyRound className="w-4 h-4 text-indigo-500" />}
          onOpenChange={onOpenChange}
          hasDivider
        />

        <VStack gap={3} className="py-4">
          <TextInput
            type="password"
            label="管理密钥"
            placeholder="输入密钥"
            value={key}
            onChange={val => setKey(val)}
            hasAutoFocus
          />
          {error && <p className="text-xs font-semibold text-error">{error}</p>}
        </VStack>

        <HStack gap={3} hAlign="end" className="pt-4 border-t border-border">
          <Button 
            type="submit" 
            variant="primary" 
            label="登录" 
            isDisabled={isSubmitting || !key.trim()} 
            isLoading={isSubmitting}
          />
        </HStack>
      </form>
    </Dialog>
  );
}
