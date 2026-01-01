import { get, post, put, del } from '../config/axios'
import type { ClientConfig, ClientStatus, ClientDetail, ServerStatus, PluginInfo, StorePluginInfo, PluginConfigResponse, JSPlugin, RuleSchemasMap } from '../types'

// 重新导出 token 管理方法
export { getToken, setToken, removeToken } from '../config/axios'

// 认证 API
export const login = (username: string, password: string) =>
  post<{ token: string }>('/auth/login', { username, password })
export const checkAuth = () => get('/auth/check')

// 服务器状态
export const getServerStatus = () => get<ServerStatus>('/status')

// 客户端管理
export const getClients = () => get<ClientStatus[]>('/clients')
export const getClient = (id: string) => get<ClientDetail>(`/client/${id}`)
export const addClient = (client: ClientConfig) => post('/clients', client)
export const updateClient = (id: string, client: ClientConfig) => put(`/client/${id}`, client)
export const deleteClient = (id: string) => del(`/client/${id}`)
export const reloadConfig = () => post('/config/reload')

// 客户端控制
export const pushConfigToClient = (id: string) => post(`/client/${id}/push`)
export const disconnectClient = (id: string) => post(`/client/${id}/disconnect`)
export const restartClient = (id: string) => post(`/client/${id}/restart`)
export const installPluginsToClient = (id: string, plugins: string[]) =>
  post(`/client/${id}/install-plugins`, { plugins })

// 规则配置模式
export const getRuleSchemas = () => get<RuleSchemasMap>('/rule-schemas')

// 客户端插件控制
export const stopClientPlugin = (clientId: string, pluginName: string, ruleName: string) =>
  post(`/client/${clientId}/plugin/${pluginName}/stop`, { rule_name: ruleName })
export const restartClientPlugin = (clientId: string, pluginName: string, ruleName: string) =>
  post(`/client/${clientId}/plugin/${pluginName}/restart`, { rule_name: ruleName })
export const updateClientPluginConfigWithRestart = (clientId: string, pluginName: string, ruleName: string, config: Record<string, string>, restart: boolean) =>
  post(`/client/${clientId}/plugin/${pluginName}/config`, { rule_name: ruleName, config, restart })

// 插件管理
export const getPlugins = () => get<PluginInfo[]>('/plugins')
export const enablePlugin = (name: string) => post(`/plugin/${name}/enable`)
export const disablePlugin = (name: string) => post(`/plugin/${name}/disable`)

// 扩展商店
export const getStorePlugins = () => get<{ plugins: StorePluginInfo[] }>('/store/plugins')
export const installStorePlugin = (pluginName: string, downloadUrl: string, signatureUrl: string, clientId: string) =>
  post('/store/install', { plugin_name: pluginName, download_url: downloadUrl, signature_url: signatureUrl, client_id: clientId })

// 客户端插件配置
export const getClientPluginConfig = (clientId: string, pluginName: string) =>
  get<PluginConfigResponse>(`/client-plugin/${clientId}/${pluginName}/config`)
export const updateClientPluginConfig = (clientId: string, pluginName: string, config: Record<string, string>) =>
  put(`/client-plugin/${clientId}/${pluginName}/config`, { config })

// JS 插件管理
export const getJSPlugins = () => get<JSPlugin[]>('/js-plugins')
export const createJSPlugin = (plugin: JSPlugin) => post('/js-plugins', plugin)
export const getJSPlugin = (name: string) => get<JSPlugin>(`/js-plugin/${name}`)
export const updateJSPlugin = (name: string, plugin: JSPlugin) => put(`/js-plugin/${name}`, plugin)
export const deleteJSPlugin = (name: string) => del(`/js-plugin/${name}`)
export const pushJSPluginToClient = (pluginName: string, clientId: string) =>
  post(`/js-plugin/${pluginName}/push/${clientId}`)
export const updateJSPluginConfig = (name: string, config: Record<string, string>) =>
  put(`/js-plugin/${name}/config`, { config })
export const setJSPluginEnabled = (name: string, enabled: boolean) =>
  post(`/js-plugin/${name}/${enabled ? 'enable' : 'disable'}`)
