import { useState, useEffect } from "react";
import { Laptop, CheckCircle2, XCircle, Plus, RefreshCw, AlertCircle, Wifi, ShieldCheck } from "lucide-react";
import { Button } from "@astryxdesign/core/Button";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Card } from "@astryxdesign/core/Card";
import { Spinner } from "@astryxdesign/core/Spinner";
import { Badge } from "@astryxdesign/core/Badge";
import { VStack, HStack } from "@astryxdesign/core/Layout";
import { api } from "@/lib/api";

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
      setMessage({ type: "error", text: "请填写所有必填字段。" });
      return;
    }

    setIsRegistering(true);
    try {
      const res = await api.registerBrowser(connectId.trim(), deviceName.trim());
      if (res.success) {
        setMessage({ type: "success", text: `设备 "${deviceName}" 注册成功！` });
        setConnectId("");
        setDeviceName("");
        fetchConnectedBrowsers();
      } else {
        setMessage({ type: "error", text: res.detail || res.error || "注册失败" });
      }
    } catch (err) {
      console.error("Registration failed", err);
      setMessage({ type: "error", text: "注册失败，设备ID冲突或网络连接错误。" });
    } finally {
      setIsRegistering(false);
    }
  };

  const handleKick = async (connId: string, name: string) => {
    if (!confirm(`确定要强制断开设备 "${name}" (ID: ${connId}) 的 WebSocket 连接吗？`)) {
      return;
    }
    try {
      const res = await api.kickBrowser(connId);
      if (res.success) {
        fetchConnectedBrowsers();
      } else {
        alert("断开连接失败: " + (res.detail || "未知原因"));
      }
    } catch (err) {
      console.error("Failed to kick browser", err);
      alert("网络错误，断开连接失败");
    }
  };

  const handleBan = async (connId: string, name: string) => {
    if (!confirm(`确定要永久封禁设备 "${name}" (ID: ${connId}) 吗？封禁后该设备将无法再次连接。`)) {
      return;
    }
    try {
      const res = await api.banBrowser(connId);
      if (res.success) {
        fetchConnectedBrowsers();
      } else {
        alert("封禁设备失败: " + (res.detail || "未知原因"));
      }
    } catch (err) {
      console.error("Failed to ban browser", err);
      alert("网络错误，封禁设备失败");
    }
  };

  const handleUnban = async (connId: string, name: string) => {
    if (!confirm(`确定要解封设备 "${name}" (ID: ${connId}) 吗？解封后该设备将可以重新连接。`)) {
      return;
    }
    try {
      const res = await api.unbanBrowser(connId);
      if (res.success) {
        fetchConnectedBrowsers();
      } else {
        alert("解封设备失败: " + (res.detail || "未知原因"));
      }
    } catch (err) {
      console.error("Failed to unban browser", err);
      alert("网络错误，解封设备失败");
    }
  };

  return (
    <div className="flex-1 overflow-y-auto bg-body p-8">
      <VStack gap={6} className="max-w-4xl mx-auto pb-12">
        <HStack gap={3} vAlign="center" hAlign="between">
          <div>
            <h2 className="text-xl font-bold tracking-tight text-primary">设备与连接设置</h2>
            <p className="text-xs text-secondary mt-1">
              管理连接到 Grabby 服务的浏览器扩展设备，并查看当前的 WebSocket 连接状态。
            </p>
          </div>
          <Button
            onClick={fetchConnectedBrowsers}
            variant="secondary"
            size="sm"
            isDisabled={isLoading}
            isLoading={isLoading}
            label="刷新状态"
            icon={<RefreshCw className="w-3.5 h-3.5" />}
          />
        </HStack>

        <div className="grid md:grid-cols-3 gap-6">
          {/* Connection Status Summary */}
          <Card className="md:col-span-1">
            <div className="p-4 border-b border-border">
              <h4 className="text-sm font-bold flex items-center gap-2 text-primary">
                <Wifi className="w-4 h-4 text-blue-500" />
                服务状态
              </h4>
            </div>
            <div className="p-4">
              <VStack gap={4}>
                <HStack gap={2} hAlign="between" vAlign="center" className="p-3 bg-muted rounded-xl border border-border">
                  <span className="text-xs font-medium text-zinc-500">连接设备数量</span>
                  <span className="text-lg font-bold">{browsers.length}</span>
                </HStack>
                <HStack gap={2} vAlign="center" className="text-xs">
                  {browsers.length > 0 ? (
                    <>
                      <CheckCircle2 className="w-4 h-4 text-success shrink-0" />
                      <span className="text-success font-semibold">
                        已建立浏览器通信
                      </span>
                    </>
                  ) : (
                    <>
                      <XCircle className="w-4 h-4 text-error shrink-0" />
                      <span className="text-error font-semibold">
                        暂无活跃连接设备
                      </span>
                    </>
                  )}
                </HStack>
                <VStack gap={1} className="p-3 border border-warning/10 bg-warning/5 rounded-xl text-[10px] text-zinc-500 dark:text-zinc-400 leading-relaxed">
                  <div className="font-semibold text-warning flex items-center gap-1">
                    <ShieldCheck className="w-3.5 h-3.5" />
                    提示
                  </div>
                  <span>使用 Grabby Web 网页抓取功能时，必须先在浏览器中安装并启用 Chrome 扩展，并将扩展中的 ID 与服务注册 ID 保持一致。</span>
                </VStack>
              </VStack>
            </div>
          </Card>

          {/* Connected Device List */}
          <Card className="md:col-span-2 overflow-hidden">
            <div className="border-b border-border p-6 bg-muted">
              <h4 className="text-base font-bold text-primary">已注册的设备列表 ({browsers.length})</h4>
              <p className="text-xs text-secondary mt-1">
                管理已在服务中登记的浏览器扩展设备，可执行断开连接和封禁操作。
              </p>
            </div>
            
            {browsers.length === 0 ? (
              <VStack gap={2} hAlign="center" vAlign="center" className="p-12 text-zinc-400 dark:text-zinc-500">
                <Laptop className="w-10 h-10 mx-auto mb-3 opacity-20" />
                <p className="text-xs font-semibold">没有检测到已注册的设备</p>
                <p className="text-[10px] mt-1">请在下方表单中注册一个新的设备 ID。</p>
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
                            <Badge variant="error" label="已封禁" />
                          ) : b.online ? (
                            <Badge variant="success" label="在线" />
                          ) : (
                            <Badge variant="neutral" label="离线" />
                          )}
                        </HStack>
                        <p className="text-[10px] text-zinc-400 font-mono mt-1 select-all">{b.conn_id}</p>
                      </div>
                    </HStack>
                    <HStack gap={3} vAlign="center">
                      <span className="text-[10px] text-zinc-400 dark:text-zinc-500 font-medium hidden sm:inline">
                        {b.banned ? "已禁止连接" : b.online ? "实时抓取就绪" : "等待连接"}
                      </span>
                      {isAuthenticated && (
                        <HStack gap={1.5} vAlign="center">
                          {b.banned ? (
                            <Button
                              onClick={() => handleUnban(b.conn_id, b.name)}
                              size="sm"
                              variant="ghost"
                              label="解封"
                            />
                          ) : (
                            <>
                              {b.online && (
                                <Button
                                  onClick={() => handleKick(b.conn_id, b.name)}
                                  size="sm"
                                  variant="ghost"
                                  label="断开"
                                />
                              )}
                              <Button
                                onClick={() => handleBan(b.conn_id, b.name)}
                                size="sm"
                                variant="ghost"
                                label="封禁"
                              />
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
                注册新设备/浏览器
              </h4>
              <p className="text-xs text-secondary mt-1">
                在 Grabby 服务中登记一个专用的设备连接标识（Connect ID），只有注册的设备 ID 才可以建立连接并接收网页抓取任务。
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
                      label="设备连接 ID (Connect ID) *"
                      placeholder="例如：ebb35609-5aef-472a-a4fd-50cbea38d8e4"
                      value={connectId}
                      onChange={setConnectId}
                      isRequired
                    />
                    <p className="text-[10px] text-zinc-400 leading-normal">
                      可在 Chrome 扩展设置面板中复制此 UUID 标识。
                    </p>
                  </VStack>

                  <VStack gap={1}>
                    <TextInput
                      label="设备名称 (Device Name) *"
                      placeholder="例如：brave, chrome-mac"
                      value={deviceName}
                      onChange={setDeviceName}
                      isRequired
                    />
                    <p className="text-[10px] text-zinc-400 leading-normal">
                      为此设备指定一个易读的标识名称。
                    </p>
                  </VStack>
                </div>

                <div className="pt-2 flex justify-end">
                  <Button 
                    type="submit" 
                    isDisabled={isRegistering}
                    isLoading={isRegistering}
                    variant="primary"
                    label="确认注册此设备"
                    icon={<Plus className="w-3.5 h-3.5" />}
                  />
                </div>
              </form>
            </div>
          </Card>
        )}
      </VStack>
    </div>
  );
}
