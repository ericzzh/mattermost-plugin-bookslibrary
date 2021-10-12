import { createSlice, createAsyncThunk } from "@reduxjs/toolkit";
import manifest from "./manifest";
import { Client4 } from "mattermost-redux/client";
import { GlobalState } from "mattermost-redux/types/store";

//@ts-ignore GlobalState is not complete
const pluginState = (state: GlobalState) => state['plugins-' + manifest.id] as state || {};

export const getExpireDays = (state:GlobalState)=>pluginState(state).expire_days
export const getMaxRenewTimes = (state:GlobalState)=>pluginState(state).max_renew_times
export const getStatus = (state:GlobalState)=>pluginState(state).status
export const getError = (state:GlobalState)=>pluginState(state).error

export const fetchConfig = createAsyncThunk(
    `${manifest.id}/config`,
    async () => {
        const data = await Client4.doFetch<Result>(
            `/plugins/${manifest.id}/config`,
            {
                method: "GET",
            }
        );
        return data;
    }
);

interface state{
    expire_days:number,
    max_renew_times:number,
    status: 'idle' | 'loading' | 'succeeded' | 'failed',
    error:string,
  }

const ConfigSlice = createSlice({
    name: manifest.id,
    initialState :{
        expire_days: -1,
        max_renew_times: -1,
        status:"idle",
        error:"",
    } as state ,
    reducers: {
//         setExpireDay(state, action) {
//             state.expire_date = action.payload;
//         },
// 
//         setMaxRenewTimes(state, action) {
//             state.max_renew_times = action.payload;
//         },
    },
    extraReducers(builder){
          builder
          .addCase(fetchConfig.pending, (state,action)=>{
                state.status = "loading"
            })
          .addCase(fetchConfig.fulfilled,(state, action)=>{
                const config = JSON.parse(action.payload.messages["data"])
                const error = action.payload.error

                state.status = "succeeded"
                
                state.expire_days = config.expire_days
                state.max_renew_times = config.max_renew_times
                state.error = error

            })
          .addCase(fetchConfig.rejected, (state,action)=>{
                state.status = "failed"
            })
      }
});

export default ConfigSlice.reducer


