#!/usr/bin/env python3
"""
Quick test script to verify all functionality works
"""

from pygofastproxy import run_proxy
import time
import requests

def test_basic_functionality():
    print("Testing basic proxy functionality...")
    
    # Start proxy
    proc = run_proxy(
        target="http://httpbin.org",  # Use httpbin.org as test backend
        port=8082,
        enable_metrics=True,
        rate_limit_rps=100
    )
    
    time.sleep(2)  # Wait for proxy to start
    
    try:
        # Test basic request
        response = requests.get("http://localhost:8082/get", timeout=5)
        print(f"Basic GET request: {response.status_code}")
        
        # Test metrics endpoint  
        response = requests.get("http://localhost:8082/__proxy_metrics", timeout=5)
        if response.status_code == 200:
            metrics = response.json() 
            print(f"Metrics endpoint: {metrics}")
        else:
            print(f"Metrics endpoint failed: {response.status_code}")
            
        # Test rate limiting (make many requests quickly)
        print("Testing rate limiting...")
        success_count = 0
        rate_limited_count = 0
        
        for i in range(20):
            try:
                resp = requests.get("http://localhost:8082/get", timeout=1)
                if resp.status_code == 200:
                    success_count += 1
                elif resp.status_code == 429:
                    rate_limited_count += 1
            except:
                pass
                
        print(f"Rate limiting test: {success_count} success, {rate_limited_count} rate limited")
        
        # Final metrics
        response = requests.get("http://localhost:8082/__proxy_metrics", timeout=5)
        if response.status_code == 200:
            final_metrics = response.json()
            print(f"Final metrics: {final_metrics}")
            
    except Exception as e:
        print(f"Test failed: {e}")
        
    finally:
        proc.terminate()
        print("Test completed!")

if __name__ == "__main__":
    test_basic_functionality()