/* eslint-disable react-hooks/set-state-in-effect -- legacy data views load on mount */
import { useState, useEffect } from "react";
import { Laptop, CheckCircle2, XCircle, Plus, RefreshCw, AlertCircle, Wifi, ShieldCheck } from "lucide-react";
import { Button } from "@astryxdesign/core/Button";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Card } from "@astryxdesign/core/Card";
import { Badge } from "@astryxdesign/core/Badge";
import { VStack, HStack } from "@astryxdesign/core/Layout";
import { api } from "@/lib/api";
import { useFeedback } from "@/components/ui/feedback-context";
import { PageFrame } from "@/components/ui/PageFrame";
import { useCopy } from "@/lib/copy";
import { deviceCopy } from "./copy";

interface BrowserInfo {
  conn_id: string;
  name: string;
  online: boolean;
  banned: boolean;
}

interface DeviceSettingsViewProps {
  isAuthenticated: boolean;
}

export function DeviceSettingsView({ isAuthenticated }: DeviceSettingsViewProps) {
  const { toast, confirm: ask } = useFeedback();
  const c = useCopy(deviceCopy);
  const [browsers, setBrowsers] = useState<BrowserInfo[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [connectId, setConnectId] = useState<string>("");
  const [deviceName, setDeviceName] = useState<string>("");
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);
  const [isRegistering, setIsRegistering] = useState<boolean>(false);

  const fetchConnectedBrowsers = async () => {
    setIsLoading(true);
    try {
      const data = await api.getBrowsers();
      setBrowsers(data.browsers || []);
    } catch (err) {
      console.error("Failed to fetch connected browsers", err);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchConnectedBrowsers();
  }, []);

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    setMessage(null);
    
    if (!connectId.trim() || !deviceName.trim()) {
      setMessage({ type: "error", text: c.errRequired });
      return;
    }

    setIsRegistering(true);
    try {
      const res = await api.registerBrowser(connectId.trim(), deviceName.trim());
      if (res.success) {
        setMessage({ type: "success", text: c.registered(deviceName) });
        setConnectId("");
        setDeviceName("");
        fetchConnectedBrowsers();
      } else {
        setMessage({ type: "error", text: res.detail || res.error || c.errRegisterFailed });
      }
    } catch (err) {
      console.error("Registration failed", err);
      setMessage({ type: "error", text: c.errRegisterNetwork });
    } finally {
      setIsRegistering(false);
    }
  };

  const handleKick = async (connId: string, name: string) => {
    const ok = await ask({
      title: c.confirmKickTitle,
      description: c.confirmKickDesc(name),
      confirmLabel: c.actKick,
    });
    if (!ok) return;
    try {
      const res = await api.kickBrowser(connId);
      if (res.success) {
        toast(c.confirmKickTitle, c.kicked(name), "success");
        fetchConnectedBrowsers();
      } else {
        toast(c.kickFailed, res.detail || c.unknownError, "error");
      }
    } catch (err) {
      console.error("Failed to kick browser", err);
      toast(c.networkError, c.networkKick, "error");
    }
  };

  const handleBan = async (connId: string, name: string) => {
    const ok = await ask({
      title: c.confirmBanTitle,
      description: c.confirmBanDesc(name),
      confirmLabel: c.actBan,
      destructive: true,
    });
    if (!ok) return;
    try {
      const res = await api.banBrowser(connId);
      if (res.success) {
        toast(c.confirmBanTitle, c.banned(name), "success");
        fetchConnectedBrowsers();
      } else {
        toast(c.banFailed, res.detail || c.unknownError, "error");
      }
    } catch (err) {
      console.error("Failed to ban browser", err);
      toast(c.networkError, c.networkBan, "error");
    }
  };

  const handleUnban = async (connId: string, name: string) => {
    const ok = await ask({
      title: c.confirmUnbanTitle,
      description: c.confirmUnbanDesc(name),
      confirmLabel: c.actUnban,
    });
    if (!ok) return;
    try {
      const res = await api.unbanBrowser(connId);
      if (res.success) {
        toast(c.confirmUnbanTitle, c.unbanned(name), "success");
        fetchConnectedBrowsers();
      } else {
        toast(c.unbanFailed, res.detail || c.unknownError, "error");
      }
    } catch (err) {
      console.error("Failed to unban browser", err);
      toast(c.networkError, c.networkUnban, "error");
    }
  };

  return (
    <PageFrame
      kicker={c.kicker}
      title={c.title}
      description={c.description}
      actions={
        <button className="g-btn g-btn-secondary g-btn-sm" onClick={fetchConnectedBrowsers} disabled={isLoading}>
          <RefreshCw size={12} className={isLoading ? "g-spin" : ""} /> {c.refresh}
        </button>
      }
    >
      <div className="space-y-5">
        <div className="grid md:grid-cols-3 gap-5">
          {/* Connection Status Summary */}
          <Card className="md:col-span-1">
            <div className="p-4 border-b border-border">
              <h4 className="text-sm font-bold flex items-center gap-2 text-primary">
                <Wifi className="w-4 h-4 text-blue-500" />
                {c.statusTitle}
              </h4>
            </div>
            <div className="p-4">
              <VStack gap={4}>
                <HStack gap={2} hAlign="between" vAlign="center" className="p-3 bg-muted rounded-xl border border-border">
                  <span className="text-xs font-medium text-zinc-500">{c.connectedCount}</span>
                  <span className="text-lg font-bold">{browsers.length}</span>
                </HStack>
                <HStack gap={2} vAlign="center" className="text-xs">
                  {browsers.length > 0 ? (
                    <>
                      <CheckCircle2 className="w-4 h-4 text-success shrink-0" />
                      <span className="text-success font-semibold">
                        {c.commEstablished}
                      </span>
                    </>
                  ) : (
                    <>
                      <XCircle className="w-4 h-4 text-error shrink-0" />
                      <span className="text-error font-semibold">
                        {c.commNone}
                      </span>
                    </>
                  )}
                </HStack>
                <VStack gap={1} className="p-3 border border-warning/10 bg-warning/5 rounded-xl text-[10px] text-zinc-500 dark:text-zinc-400 leading-relaxed">
                  <div className="font-semibold text-warning flex items-center gap-1">
                    <ShieldCheck className="w-3.5 h-3.5" />
                    {c.tipTitle}
                  </div>
                  <span>{c.tipBody}</span>
                </VStack>
              </VStack>
            </div>
          </Card>

          {/* Connected Device List */}
          <Card className="md:col-span-2 overflow-hidden">
            <div className="border-b border-border p-6 bg-muted">
              <h4 className="text-base font-bold text-primary">{c.listTitle(browsers.length)}</h4>
              <p className="text-xs text-secondary mt-1">
                {c.listDesc}
              </p>
            </div>
            
            {browsers.length === 0 ? (
              <VStack gap={2} hAlign="center" vAlign="center" className="p-12 text-zinc-400 dark:text-zinc-500">
                <Laptop className="w-10 h-10 mx-auto mb-3 opacity-20" />
                <p className="text-xs font-semibold">{c.emptyTitle}</p>
                <p className="text-[10px] mt-1">{c.emptyBody}</p>
              </VStack>
            ) : (
              <div className="divide-y divide-border">
                {browsers.map((b) => (
                  <div key={b.conn_id} className="p-4 flex items-center justify-between hover:bg-muted transition-colors">
                    <HStack gap={3} vAlign="center">
                      <div className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 ${
                        b.banned 
                          ? "bg-rose-50 dark:bg-rose-950/20 text-rose-500" 
                          : b.online 
                            ? "bg-blue-subtle text-accent" 
                            : "bg-zinc-100 dark:bg-zinc-900 text-zinc-400"
                      }`}>
                        <Laptop className="w-5 h-5" />
                      </div>
                      <div>
                        <HStack gap={2} vAlign="center" className="flex-wrap">
                          <h5 className="font-bold text-sm leading-none">{b.name}</h5>
                          {b.banned ? (
                            <Badge variant="error" label={c.badgeBanned} />
                          ) : b.online ? (
                            <Badge variant="success" label={c.badgeOnline} />
                          ) : (
                            <Badge variant="neutral" label={c.badgeOffline} />
                          )}
                        </HStack>
                        <p className="text-[10px] text-zinc-400 font-mono mt-1 select-all">{b.conn_id}</p>
                      </div>
                    </HStack>
                    <HStack gap={3} vAlign="center">
                      <span className="text-[10px] text-zinc-400 dark:text-zinc-500 font-medium hidden sm:inline">
                        {b.banned ? c.stateBanned : b.online ? c.stateReady : c.stateWaiting}
                      </span>
                      {isAuthenticated && (
                        <HStack gap={1.5} vAlign="center">
                          {b.banned ? (
                            <button className="g-btn g-btn-secondary g-btn-sm" onClick={() => handleUnban(b.conn_id, b.name)}>
                              {c.actUnban}
                            </button>
                          ) : (
                            <>
                              {b.online && (
                                <button className="g-btn g-btn-ghost g-btn-sm" onClick={() => handleKick(b.conn_id, b.name)}>
                                  {c.actKick}
                                </button>
                              )}
                              <button className="g-btn g-btn-danger g-btn-sm" onClick={() => handleBan(b.conn_id, b.name)}>
                                {c.actBan}
                              </button>
                            </>
                          )}
                        </HStack>
                      )}
                    </HStack>
                  </div>
                ))}
              </div>
            )}
          </Card>
        </div>

        {/* Register Device Form */}
        {isAuthenticated && (
          <Card>
            <div className="border-b border-border p-6 bg-muted">
              <h4 className="text-base font-bold flex items-center gap-2 text-primary">
                <Plus className="w-5 h-5 text-indigo-500" />
                {c.registerTitle}
              </h4>
              <p className="text-xs text-secondary mt-1">
                {c.registerDesc}
              </p>
            </div>
            <div className="p-6">
              <form onSubmit={handleRegister} className="space-y-4 max-w-xl">
                {message && (
                  <div className={`p-3 rounded-xl flex items-center gap-2 text-xs font-semibold ${
                    message.type === "success" 
                      ? "bg-emerald-50 text-emerald-700 dark:bg-emerald-950/20 dark:text-emerald-400" 
                      : "bg-rose-50 text-rose-700 dark:bg-rose-950/20 dark:text-rose-400"
                  }`}>
                    {message.type === "success" ? (
                      <CheckCircle2 className="w-4 h-4 text-success" />
                    ) : (
                      <AlertCircle className="w-4 h-4 text-error" />
                    )}
                    {message.text}
                  </div>
                )}

                <div className="grid md:grid-cols-2 gap-4">
                  <VStack gap={1}>
                    <TextInput
                      label={c.labelConnectId}
                      placeholder={c.placeholderConnectId}
                      value={connectId}
                      onChange={setConnectId}
                    />
                    <p className="text-[10px] text-zinc-400 leading-normal">
                      {c.hintConnectId}
                    </p>
                  </VStack>

                  <VStack gap={1}>
                    <TextInput
                      label={c.labelDeviceName}
                      placeholder={c.placeholderDeviceName}
                      value={deviceName}
                      onChange={setDeviceName}
                    />
                    <p className="text-[10px] text-zinc-400 leading-normal">
                      {c.hintDeviceName}
                    </p>
                  </VStack>
                </div>

                <div className="pt-2 flex justify-end">
                  <Button 
                    type="submit" 
                    isDisabled={isRegistering}
                    isLoading={isRegistering}
                    variant="primary"
                    label={c.registerSubmit}
                    icon={<Plus className="w-3.5 h-3.5" />}
                  />
                </div>
              </form>
            </div>
          </Card>
        )}
      </div>
    </PageFrame>
  );
}
