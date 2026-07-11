export namespace ai {
	
	export class Endpoint {
	    name: string;
	    baseUrl: string;
	    local: boolean;
	    running: boolean;
	    models?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Endpoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.baseUrl = source["baseUrl"];
	        this.local = source["local"];
	        this.running = source["running"];
	        this.models = source["models"];
	    }
	}
	export class ValidationResult {
	    ok: boolean;
	    local: boolean;
	    verdict: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ValidationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.local = source["local"];
	        this.verdict = source["verdict"];
	        this.message = source["message"];
	    }
	}

}

export namespace connection {
	
	export class Connection {
	    id: number;
	    provider: string;
	    accountLabel: string;
	    accountId: string;
	    scopes: string[];
	    status: string;
	    connectedAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.provider = source["provider"];
	        this.accountLabel = source["accountLabel"];
	        this.accountId = source["accountId"];
	        this.scopes = source["scopes"];
	        this.status = source["status"];
	        this.connectedAt = source["connectedAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}

}

export namespace main {
	
	export class AIClassification {
	    local: boolean;
	    verdict: string;
	
	    static createFrom(source: any = {}) {
	        return new AIClassification(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.local = source["local"];
	        this.verdict = source["verdict"];
	    }
	}
	export class GoogleAuthStatus {
	    mode: string;
	    brokerBaseUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new GoogleAuthStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.brokerBaseUrl = source["brokerBaseUrl"];
	    }
	}

}

