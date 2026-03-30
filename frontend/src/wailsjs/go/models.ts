export namespace model {
	
	export class AuthState {
	    is_authenticated: boolean;
	    login: string;
	    avatar_url: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.is_authenticated = source["is_authenticated"];
	        this.login = source["login"];
	        this.avatar_url = source["avatar_url"];
	    }
	}
	export class DeviceFlowInfo {
	    device_code: string;
	    user_code: string;
	    verification_uri: string;
	    expires_in: number;
	    interval: number;
	
	    static createFrom(source: any = {}) {
	        return new DeviceFlowInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device_code = source["device_code"];
	        this.user_code = source["user_code"];
	        this.verification_uri = source["verification_uri"];
	        this.expires_in = source["expires_in"];
	        this.interval = source["interval"];
	    }
	}
	export class PollResult {
	    status: string;
	    error?: string;
	    interval?: number;
	
	    static createFrom(source: any = {}) {
	        return new PollResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.error = source["error"];
	        this.interval = source["interval"];
	    }
	}

}

