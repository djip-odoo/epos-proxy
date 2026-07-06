export namespace main {
	
	export class LANSettings {
	    enabled: boolean;
	    ip: string;
	    port: number;
	
	    static createFrom(source: any = {}) {
	        return new LANSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.ip = source["ip"];
	        this.port = source["port"];
	    }
	}
	export class Printer {
	    name: string;
	    ip: string;
	    id: string;
	    isLAN: boolean;
	    lanIp?: string;
	    online: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Printer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ip = source["ip"];
	        this.id = source["id"];
	        this.isLAN = source["isLAN"];
	        this.lanIp = source["lanIp"];
	        this.online = source["online"];
	    }
	}
	export class UnavailablePrinter {
	    name: string;
	    errorMsg: string;
	    isLAN: boolean;
	    lanIp?: string;
	
	    static createFrom(source: any = {}) {
	        return new UnavailablePrinter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.errorMsg = source["errorMsg"];
	        this.isLAN = source["isLAN"];
	        this.lanIp = source["lanIp"];
	    }
	}
	export class Status {
	    serverRunning: boolean;
	    errorMsg: string;
	    printers: Printer[];
	    unavailablePrinters: UnavailablePrinter[];
	    os: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serverRunning = source["serverRunning"];
	        this.errorMsg = source["errorMsg"];
	        this.printers = this.convertValues(source["printers"], Printer);
	        this.unavailablePrinters = this.convertValues(source["unavailablePrinters"], UnavailablePrinter);
	        this.os = source["os"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

