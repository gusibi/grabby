import { useState, useEffect } from "react";
import { Laptop, CheckCircle2, XCircle, Plus, RefreshCw, AlertCircle, Wifi, ShieldCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";

interface BrowserInfo {
  conn_id: string;
  name: string;
}

export function DeviceSettingsView() {
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
        // Refresh connected list
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

  return (
    <div className="flex-1 overflow-y-auto bg-zinc-50/50 dark:bg-[#121212] p-8">
      <div className="max-w-4xl mx-auto space-y-8 pb-12">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-xl font-bold tracking-tight">设备与连接设置</h2>
            <p className="text-xs text-zinc-500 dark:text-zinc-400">
              管理连接到 Grabby 服务的浏览器扩展设备，并查看当前的 WebSocket 连接状态。
            </p>
          </div>
          <Button
            onClick={fetchConnectedBrowsers}
            variant="outline"
            size="sm"
            disabled={isLoading}
            className="gap-1.5 h-8 text-xs font-semibold"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${isLoading ? "animate-spin" : ""}`} />
            刷新状态
          </Button>
        </div>

        <div className="grid md:grid-cols-3 gap-6">
          {/* Connection Status Summary */}
          <Card className="md:col-span-1 border border-black/5 dark:border-white/5 bg-white dark:bg-[#1c1c1e] shadow-sm rounded-2xl">
            <CardHeader className="pb-3">
              <CardTitle className="text-sm font-bold flex items-center gap-2">
                <Wifi className="w-4 h-4 text-blue-500" />
                服务状态
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                <div className="flex items-center justify-between p-3 bg-zinc-50 dark:bg-zinc-900/50 rounded-xl">
                  <span className="text-xs font-medium text-zinc-500">连接设备数量</span>
                  <span className="text-lg font-bold">{browsers.length}</span>
                </div>
                <div className="flex items-center gap-2 text-xs">
                  {browsers.length > 0 ? (
                    <>
                      <CheckCircle2 className="w-4 h-4 text-emerald-500 shrink-0" />
                      <span className="text-emerald-600 dark:text-emerald-400 font-semibold">
                        已建立浏览器通信
                      </span>
                    </>
                  ) : (
                    <>
                      <XCircle className="w-4 h-4 text-rose-500 shrink-0" />
                      <span className="text-rose-600 dark:text-rose-400 font-semibold">
                        暂无活跃连接设备
                      </span>
                    </>
                  )}
                </div>
                <div className="p-3 border border-yellow-500/10 bg-yellow-500/5 rounded-xl text-[10px] text-zinc-500 dark:text-zinc-400 space-y-1 leading-relaxed">
                  <div className="font-semibold text-yellow-600 dark:text-yellow-500 flex items-center gap-1">
                    <ShieldCheck className="w-3.5 h-3.5" />
                    提示
                  </div>
                  使用 Grabby Web 网页抓取功能时，必须先在浏览器中安装并启用 Chrome 扩展，并将扩展中的 ID 与服务注册 ID 保持一致。
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Connected Device List */}
          <Card className="md:col-span-2 border border-black/5 dark:border-white/5 bg-white dark:bg-[#1c1c1e] shadow-sm rounded-2xl overflow-hidden">
            <CardHeader className="border-b border-black/5 dark:border-white/5 p-6 bg-zinc-50/50 dark:bg-zinc-900/50">
              <CardTitle className="text-base font-bold">当前连接中的设备 ({browsers.length})</CardTitle>
              <CardDescription className="text-xs">
                正在通过 WebSocket 保持心跳连接的实时浏览器实例。
              </CardDescription>
            </CardHeader>
            <CardContent className="p-0">
              {browsers.length === 0 ? (
                <div className="p-12 text-center text-zinc-400 dark:text-zinc-500">
                  <Laptop className="w-10 h-10 mx-auto mb-3 opacity-20" />
                  <p className="text-xs font-semibold">没有检测到已连接的设备</p>
                  <p className="text-[10px] mt-1">请启动 Chrome 浏览器并启用 Grabby 扩展程序。</p>
                </div>
              ) : (
                <div className="divide-y divide-black/5 dark:divide-white/5">
                  {browsers.map((b) => (
                    <div key={b.conn_id} className="p-4 flex items-center justify-between hover:bg-zinc-50/50 dark:hover:bg-zinc-900/20 transition-colors">
                      <div className="flex items-center gap-3">
                        <div className="w-10 h-10 rounded-xl bg-blue-50 dark:bg-zinc-800 text-blue-500 flex items-center justify-center shrink-0">
                          <Laptop className="w-5 h-5" />
                        </div>
                        <div>
                          <div className="flex items-center gap-2">
                            <h5 className="font-bold text-sm leading-none">{b.name}</h5>
                            <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
                            <span className="text-[9px] text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-950/20 px-1.5 py-0.2 rounded font-bold">
                              在线
                            </span>
                          </div>
                          <p className="text-[10px] text-zinc-400 font-mono mt-1 select-all">{b.conn_id}</p>
                        </div>
                      </div>
                      <div className="flex items-center gap-3">
                        <div className="text-[10px] text-zinc-400 dark:text-zinc-500 font-medium">
                          实时抓取就绪
                        </div>
                        <Button
                          onClick={() => handleKick(b.conn_id, b.name)}
                          size="sm"
                          variant="ghost"
                          className="h-7 text-xs text-rose-500 hover:text-rose-600 hover:bg-rose-50 dark:hover:bg-rose-950/20 px-2 rounded-lg gap-1"
                        >
                          断开
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        {/* Register Device Form */}
        <Card className="border border-black/5 dark:border-white/5 bg-white dark:bg-[#1c1c1e] shadow-sm rounded-2xl">
          <CardHeader>
            <CardTitle className="text-base font-bold flex items-center gap-2">
              <Plus className="w-5 h-5 text-indigo-500" />
              注册新设备/浏览器
            </CardTitle>
            <CardDescription className="text-xs">
              在 Grabby 服务中登记一个专用的设备连接标识（Connect ID），只有注册的设备 ID 才可以建立连接并接收网页抓取任务。
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleRegister} className="space-y-4 max-w-xl">
              {message && (
                <div className={`p-3 rounded-xl flex items-center gap-2 text-xs font-semibold ${
                  message.type === "success" 
                    ? "bg-emerald-50 text-emerald-700 dark:bg-emerald-950/20 dark:text-emerald-400" 
                    : "bg-rose-50 text-rose-700 dark:bg-rose-950/20 dark:text-rose-400"
                }`}>
                  {message.type === "success" ? (
                    <CheckCircle2 className="w-4 h-4 text-emerald-500" />
                  ) : (
                    <AlertCircle className="w-4 h-4 text-rose-500" />
                  )}
                  {message.text}
                </div>
              )}

              <div className="grid md:grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <label className="text-xs font-bold text-zinc-600 dark:text-zinc-400">
                    设备连接 ID (Connect ID) *
                  </label>
                  <Input
                    placeholder="例如：ebb35609-5aef-472a-a4fd-50cbea38d8e4"
                    value={connectId}
                    onChange={(e) => setConnectId(e.target.value)}
                    className="h-9 text-xs"
                    required
                  />
                  <p className="text-[10px] text-zinc-400 leading-normal">
                    可在 Chrome 扩展设置面板中复制此 UUID 标识。
                  </p>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-bold text-zinc-600 dark:text-zinc-400">
                    设备名称 (Device Name) *
                  </label>
                  <Input
                    placeholder="例如：brave, chrome-mac, office-pc"
                    value={deviceName}
                    onChange={(e) => setDeviceName(e.target.value)}
                    className="h-9 text-xs"
                    required
                  />
                  <p className="text-[10px] text-zinc-400 leading-normal">
                    为此设备指定一个易读的标识名称。
                  </p>
                </div>
              </div>

              <div className="pt-2 flex justify-end">
                <Button 
                  type="submit" 
                  disabled={isRegistering}
                  className="bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs h-9 px-4 rounded-xl gap-1.5 shadow-md shadow-indigo-500/10"
                >
                  {isRegistering ? (
                    <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                  ) : (
                    <Plus className="w-3.5 h-3.5" />
                  )}
                  确认注册此设备
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
