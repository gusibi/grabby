import * as React from "react";
import { KeyRound } from "lucide-react";
import { Dialog, DialogHeader } from "@astryxdesign/core/Dialog";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Button } from "@astryxdesign/core/Button";
import { VStack, HStack } from "@astryxdesign/core/Layout";
import { api } from "@/lib/api";
import { useCopy } from "@/lib/copy";
import { authCopy } from "./copy";

interface AuthDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onAuthenticated: () => void;
}

export function AuthDialog({ open, onOpenChange, onAuthenticated }: AuthDialogProps) {
  const c = useCopy(authCopy);
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
        setError(data.error || c.errInvalidKey);
      }
    } catch (err) {
      console.error("Login failed", err);
      setError(c.errNetwork);
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
          title={c.title}
          subtitle={c.subtitle}
          startContent={<KeyRound className="w-4 h-4 text-indigo-500" />}
          onOpenChange={onOpenChange}
          hasDivider
        />

        <VStack gap={3} className="py-4">
          <TextInput
            type="password"
            label={c.label}
            placeholder={c.placeholder}
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
            label={c.submit} 
            isDisabled={isSubmitting || !key.trim()} 
            isLoading={isSubmitting}
          />
        </HStack>
      </form>
    </Dialog>
  );
}
