export interface PluginRegistry {
    registerPostTypeComponent(typeName: string, component: React.ElementType)
    registerReducer(reducer:any);


    // Add more if needed from https://developers.mattermost.com/extend/plugins/webapp/reference
}
